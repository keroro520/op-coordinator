package internal

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/node-real/op-coordinator/internal/metrics"
	"go.uber.org/zap"
	"sync"
	"time"
)

type Coordinator struct {
	config Config

	master string
	nodes  map[string]*Node

	healthChecker *HealthChecker

	adminCh chan AdminCommand
}

func NewCoordinator(config Config) (*Coordinator, error) {
	c := Coordinator{
		config: config,
		nodes:  make(map[string]*Node),
		healthChecker: NewHealthChecker(
			time.Duration(config.HealthCheck.IntervalMs)*time.Millisecond,
			config.HealthCheck.FailureThresholdLast5,
		),
		adminCh: make(chan AdminCommand),
	}

	// Create clients for nodes
	var err error
	for nodeName, nodeCfg := range config.Candidates {
		c.nodes[nodeName], err = NewNode(nodeName, nodeCfg.OpNodePublicRpcUrl, nodeCfg.OpGethPublicRpcUrl)
		if err != nil {
			return nil, err
		}
	}
	for nodeName, nodeCfg := range config.Bridges {
		c.nodes[nodeName], err = NewNode(nodeName, nodeCfg.OpNodePublicRpcUrl, nodeCfg.OpGethPublicRpcUrl)
		if err != nil {
			return nil, err
		}
	}

	return &c, nil
}

func (c *Coordinator) Start(ctx context.Context) {
	go c.healthChecker.Start(ctx, &c.nodes)

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
			if c.master == "" {
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

			if !c.healthChecker.IsHealthy(c.master) {
				c.revokeCurrentMaster()
			} else {
				if lastMasterCheck.Add(3 * time.Second).Before(time.Now()) {
					if stopped, err := c.nodes[c.master].opNode.SequencerStopped(ctx); err == nil && stopped {
						// In the case that the master node has been restarted, it loses `SequencerStopped=false` state,
						// so we have to set `SequencerStopped=false` to reuse it as master.
						previous := c.master
						c.master = ""
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

// hasSufficientHealthyNodes checks if there are sufficient healthy nodes to elect a new master.
func (c *Coordinator) hasSufficientHealthyNodes() bool {
	healthyCandidates := 0
	for _, node := range c.nodes {
		if /* c.isCandidate(node.name) && */ c.healthChecker.IsHealthy(node.name) {
			healthyCandidates++
		}
	}

	return healthyCandidates >= c.config.Election.MinRequiredHealthyNodes
}

// revokeCurrentMaster revokes the leadership of the current master.
func (c *Coordinator) revokeCurrentMaster() {
	if c.master == "" {
		return
	}

	zap.S().Warnf("Revoke master %s", c.master)

	// Stop the sequencer by calling admin_stopSequencer
	// It's fine even if the call fails because the leadership will be revoked anyway and the node is unable to
	// produce blocks.
	client := c.nodes[c.master]
	if _, err := client.opNode.StopSequencer(context.Background()); err != nil {
		zap.S().Errorw("Fail to call admin_stopSequencer even though its leadership will be revoked", "node", c.master, "error", err)
	}

	c.master = ""
}

func (c *Coordinator) elect() error {
	zap.S().Info("Start election")
	if existingMaster := c.findExistingMaster(); existingMaster != nil {
		c.master = existingMaster.name
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.config.Election.MaxWaitingTimeForConvergenceMs)*time.Millisecond)
	defer cancel()
	_ = c.waitForConvergence(ctx)

	canonical, err := c.findCanonicalCandidate()
	if canonical == nil {
		return fmt.Errorf("fail to find canonical candidate, error: %s", err)
	}

	return c.setMaster(canonical.name)
}

func (c *Coordinator) setMaster(nodeName string) error {
	if !c.isCandidate(nodeName) {
		return errors.New("node is not a candidate")
	}
	if c.master == nodeName {
		return errors.New("node is already the master")
	}

	canonical := c.nodes[nodeName]
	maxHeight := c.findMaxHeight()
	canonicalStatus, err := canonical.opNode.SyncStatus(context.Background())
	if err != nil {
		return fmt.Errorf("fail to call optimism_syncStatus, node: %s, error: %s", canonical.name, err)
	}
	if canonicalStatus.UnsafeL2.Number < maxHeight {
		return fmt.Errorf("new master height is lower than others, node: %s, maxHeight: %d", canonical.name, maxHeight)
	}

	if err = canonical.opNode.StartSequencer(context.Background(), canonicalStatus.UnsafeL2.Hash); err != nil {
		return fmt.Errorf("fail to call admin_startSequencer, node: %s, error: %s", canonical.name, err)
	}

	c.master = nodeName
	zap.S().Infow("Success to elect new master", "node", c.master, "unsafe_l2", canonicalStatus.UnsafeL2)
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
	var resultCh = make(chan uint64, len(c.nodes))
	var wg sync.WaitGroup

	for _, node := range c.nodes {
		if !c.healthChecker.IsHealthy(node.name) {
			continue
		}

		wg.Add(1)
		go func(node *Node) {
			defer wg.Done()

			if syncStatus, err := node.opNode.SyncStatus(context.Background()); err == nil {
				resultCh <- syncStatus.UnsafeL2.Number
			} else {
				zap.S().Errorw("Fail to call optimism_syncStatus", "node", node.name, "error", err)
			}
		}(node)
	}
	wg.Wait()

	for unsafeL2 := range resultCh {
		if convergence == 0 {
			convergence = unsafeL2
		} else if convergence != unsafeL2 {
			return false
		}
		if len(resultCh) == 0 {
			break
		}
	}
	return true
}

func (c *Coordinator) isCandidate(nodeName string) bool {
	return c.config.Candidates[nodeName] != nil
}

// findCanonicalCandidate finds the candidate with the highest unsafe_l2 height
func (c *Coordinator) findCanonicalCandidate() (*Node, error) {
	var canonical *Node
	var canonicalStatus *eth.SyncStatus
	for nodeName := range c.config.Candidates {
		// Filter healthy candidates
		if !c.isCandidate(nodeName) || !c.healthChecker.IsHealthy(nodeName) {
			continue
		}

		candidate := c.nodes[nodeName]
		syncStatus, err := candidate.opNode.SyncStatus(context.Background())
		if err != nil {
			zap.S().Errorw("Fail to call optimism_syncStatus", "node", candidate.name, "error", err)
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
	for _, node := range c.nodes {
		syncStatus, err := node.opNode.SyncStatus(context.Background())
		if err == nil && (maxHeight == 0 || maxHeight < syncStatus.UnsafeL2.Number) {
			maxHeight = syncStatus.UnsafeL2.Number
		}
	}
	return maxHeight
}

// findExistingMaster returns the existing master if its admin_sequencerStopped is false.
//
// Note that this function does not check if the existing master is healthy or not. Here are considerations:
//   - If the existing master is healthy, it is okay for us to re-elect it as the master.
//   - If the existing master is unhealthy, it will be revoked by the health checker when it detects the master is
//     unhealthy and be called admin_stopSequencer. Then, we will re-elect a new master.
func (c *Coordinator) findExistingMaster() *Node {
	var found *Node
	var foundUnsafeL2 = uint64(0)
	for nodeName := range c.config.Candidates {
		if c.isCandidate(nodeName) {
			candidate := c.nodes[nodeName]
			sequencerStopped, err := candidate.opNode.SequencerStopped(context.Background())
			if err == nil && sequencerStopped == false {
				zap.S().Infow("Found existing master", "node", nodeName)

				syncStatus, err := candidate.opNode.SyncStatus(context.Background())
				if err != nil {
					zap.S().Errorw("Fail to call optimism_syncStatus", "node", candidate.name, "error", err)
					continue
				}

				if foundUnsafeL2 < syncStatus.UnsafeL2.Number {
					found = candidate
					foundUnsafeL2 = syncStatus.UnsafeL2.Number
				}
			}
		}
	}

	return found
}
