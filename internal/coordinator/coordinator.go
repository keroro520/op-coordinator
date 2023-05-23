package coordinator

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/node-real/op-coordinator/internal/config"
	"github.com/node-real/op-coordinator/internal/metrics"
	"github.com/node-real/op-coordinator/internal/types"
	"go.uber.org/zap"
	"sync"
	"time"
)

type Coordinator struct {
	Config config.Config

	Master string
	Nodes  map[string]*types.Node

	healthChecker *HealthChecker

	adminCh chan AdminCommand
}

func NewCoordinator(config config.Config) (*Coordinator, error) {
	c := Coordinator{
		Config: config,
		Nodes:  make(map[string]*types.Node),
		healthChecker: NewHealthChecker(
			time.Duration(config.HealthCheck.IntervalMs)*time.Millisecond,
			config.HealthCheck.FailureThresholdLast5,
		),
		adminCh: make(chan AdminCommand),
	}

	// Create clients for nodes
	var err error
	for nodeName, nodeCfg := range config.Candidates {
		c.Nodes[nodeName], err = types.NewNode(nodeName, nodeCfg.OpNodePublicRpcUrl, nodeCfg.OpGethPublicRpcUrl)
		if err != nil {
			return nil, err
		}
	}
	for nodeName, nodeCfg := range config.Bridges {
		c.Nodes[nodeName], err = types.NewNode(nodeName, nodeCfg.OpNodePublicRpcUrl, nodeCfg.OpGethPublicRpcUrl)
		if err != nil {
			return nil, err
		}
	}

	return &c, nil
}

func (c *Coordinator) Start(ctx context.Context) {
	go c.healthChecker.Start(ctx, &c.Nodes)

	zap.S().Info("Coordinator start")
	c.loop(ctx)
	zap.S().Info("Coordinator exit")
}

func (c *Coordinator) AdminCh() chan AdminCommand {
	return c.adminCh
}

func (c *Coordinator) loop(ctx context.Context) {
	lastMasterCheck := time.Now()
	ticker := time.NewTicker(50 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.Config.Election.Stopped {
				continue
			}
			if c.Master == "" {
				if c.hasSufficientHealthyNodes() {
					start := time.Now()
					var err error
					if err = c.elect(); err != nil {
						zap.S().Errorw("Fail to elect master", "error", err)
						metrics.MetricElectionFailures.Inc()
					}

					duration := time.Since(start)
					metrics.MetricElections.Inc()
					metrics.MetricElectionDuration.Observe(float64(duration.Milliseconds()))
				}
				continue
			}

			if !c.healthChecker.IsHealthy(c.Master) {
				c.revokeCurrentMaster()
			} else {
				if lastMasterCheck.Add(3 * time.Second).Before(time.Now()) {
					if stopped, err := c.Nodes[c.Master].OpNode.SequencerStopped(ctx); err == nil && stopped {
						// In the case that the master node has been restarted, it loses `SequencerStopped=false` state,
						// so we have to set `SequencerStopped=false` to reuse it as master.
						previous := c.Master
						c.Master = ""
						c.setMaster(previous)
					}

					lastMasterCheck = time.Now()
				}
			}
		case cmd := <-c.adminCh:
			cmd.Execute(c)
		}
	}
}

// hasSufficientHealthyNodes checks if there are sufficient healthy nodes to elect a new Master.
func (c *Coordinator) hasSufficientHealthyNodes() bool {
	healthyCandidates := 0
	for _, node := range c.Nodes {
		if /* c.isCandidate(node.name) && */ c.healthChecker.IsHealthy(node.Name) {
			healthyCandidates++
		}
	}

	return healthyCandidates >= c.Config.Election.MinRequiredHealthyNodes
}

// revokeCurrentMaster revokes the leadership of the current Master.
func (c *Coordinator) revokeCurrentMaster() {
	if c.Master == "" {
		return
	}

	zap.S().Warnf("Revoke master %s", c.Master)

	// Stop the sequencer by calling admin_stopSequencer
	// It's fine even if the call fails because the leadership will be revoked anyway and the node is unable to
	// produce blocks.
	client := c.Nodes[c.Master]
	if _, err := client.OpNode.StopSequencer(context.Background()); err != nil {
		zap.S().Errorw("Fail to call admin_stopSequencer even though its leadership will be revoked", "node", c.Master, "error", err)
	}

	c.Master = ""
}

func (c *Coordinator) elect() error {
	zap.S().Info("Start election")
	if existingMaster := c.findExistingMaster(); existingMaster != nil {
		c.Master = existingMaster.Name
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Config.Election.MaxWaitingTimeForConvergenceMs)*time.Millisecond)
	defer cancel()
	_ = c.waitForConvergence(ctx)

	canonical, err := c.findCanonicalCandidate()
	if canonical == nil {
		return fmt.Errorf("fail to find canonical candidate, error: %s", err)
	}

	return c.setMaster(canonical.Name)
}

func (c *Coordinator) setMaster(nodeName string) error {
	if !c.isCandidate(nodeName) {
		return errors.New("node is not a candidate")
	}
	if c.Master == nodeName {
		return errors.New("node is already the master")
	}

	canonical := c.Nodes[nodeName]
	maxHeight := c.findMaxHeight()
	canonicalStatus, err := canonical.OpNode.SyncStatus(context.Background())
	if err != nil {
		return fmt.Errorf("fail to call optimism_syncStatus, node: %s, error: %s", canonical.Name, err)
	}
	if canonicalStatus.UnsafeL2.Number < maxHeight {
		return fmt.Errorf("new master height is lower than others, node: %s, maxHeight: %d", canonical.Name, maxHeight)
	}

	if err = canonical.OpNode.StartSequencer(context.Background(), canonicalStatus.UnsafeL2.Hash); err != nil {
		return fmt.Errorf("fail to call admin_startSequencer, node: %s, error: %s", canonical.Name, err)
	}

	c.Master = nodeName
	zap.S().Infow("Success to elect new master", "node", c.Master, "unsafe_l2", canonicalStatus.UnsafeL2)
	return nil
}

func (c *Coordinator) waitForConvergence(ctx context.Context) bool {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		metrics.MetricWaitingConvergenceDuration.Observe(float64(duration.Milliseconds()))
		zap.S().Infow("Wait nodes to converge on the same height", "elapsed", duration)
	}()

	ticker := time.NewTicker(10 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			zap.S().Warn("Timeout waiting for nodes to converge on the same height")
			return false
		case <-ticker.C:
			if c.nodesConverged() {
				zap.S().Infof("Candidates have converged on the same height")
				return true
			}
		}
	}
}

// nodesConverged checks if all healthy nodes have the same unsafe_l2 height
func (c *Coordinator) nodesConverged() bool {
	var convergence uint64 = 0
	var convergenceFlag = true
	var convergenceLock sync.Mutex
	var wg sync.WaitGroup

	wg.Add(len(c.Nodes))
	for _, node := range c.Nodes {
		go func(node *types.Node) {
			defer wg.Done()
			if syncStatus, err := node.OpNode.SyncStatus(context.Background()); err == nil {
				convergenceLock.Lock()
				defer convergenceLock.Unlock()
				if convergence == 0 {
					convergence = syncStatus.UnsafeL2.Number
				} else if convergence != syncStatus.UnsafeL2.Number {
					convergenceFlag = false
				}
			}
		}(node)
	}
	wg.Wait()

	return convergenceFlag
}

func (c *Coordinator) isCandidate(nodeName string) bool {
	return c.Config.Candidates[nodeName] != nil
}

// findCanonicalCandidate finds the candidate with the highest unsafe_l2 height
func (c *Coordinator) findCanonicalCandidate() (*types.Node, error) {
	var canonical *types.Node
	var canonicalStatus *eth.SyncStatus
	for nodeName := range c.Config.Candidates {
		// Filter healthy candidates
		if !c.isCandidate(nodeName) || !c.healthChecker.IsHealthy(nodeName) {
			continue
		}

		candidate := c.Nodes[nodeName]
		syncStatus, err := candidate.OpNode.SyncStatus(context.Background())
		if err != nil {
			zap.S().Errorw("Fail to call optimism_syncStatus", "node", candidate.Name, "error", err)
			continue
		}

		if canonicalStatus == nil || canonicalStatus.UnsafeL2.Number < syncStatus.UnsafeL2.Number {
			canonical = candidate
			canonicalStatus = syncStatus
		}
	}

	if canonical == nil {
		return nil, errors.New("no healthy candidates")
	}
	return canonical, nil
}

func (c *Coordinator) findMaxHeight() uint64 {
	var maxHeight uint64 = 0
	var maxHeightLock sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(c.Nodes))
	for _, node := range c.Nodes {
		go func(node *types.Node) {
			defer wg.Done()
			if syncStatus, err := node.OpNode.SyncStatus(context.Background()); err == nil {
				maxHeightLock.Lock()
				defer maxHeightLock.Unlock()
				if maxHeight == 0 || maxHeight < syncStatus.UnsafeL2.Number {
					maxHeight = syncStatus.UnsafeL2.Number
				}
			}
		}(node)
	}
	wg.Wait()
	return maxHeight
}

// findExistingMaster returns the existing master if its admin_sequencerStopped is false.
//
// Note that this function does not check if the existing master is healthy or not. Here are considerations:
//   - If the existing master is healthy, it is okay for us to re-elect it as the Master.
//   - If the existing master is unhealthy, it will be revoked by the health checker when it detects the Master is
//     unhealthy and be called admin_stopSequencer. Then, we will re-elect a new Master.
func (c *Coordinator) findExistingMaster() *types.Node {
	var found *types.Node
	var foundUnsafeL2 = uint64(0)
	var foundLock sync.Mutex
	var wg sync.WaitGroup

	wg.Add(len(c.Config.Candidates))
	for nodeName := range c.Config.Candidates {
		candidate := c.Nodes[nodeName]

		go func(candidate *types.Node) {
			defer wg.Done()
			sequencerStopped, err := candidate.OpNode.SequencerStopped(context.Background())
			if err == nil && sequencerStopped == false {
				syncStatus, err := candidate.OpNode.SyncStatus(context.Background())
				if err == nil {
					foundLock.Lock()
					defer foundLock.Unlock()
					if foundUnsafeL2 < syncStatus.UnsafeL2.Number {
						found = candidate
						foundUnsafeL2 = syncStatus.UnsafeL2.Number
					}
				}
			}
		}(candidate)
	}
	wg.Wait()

	return found
}
