package internal

import (
	"context"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
	"sync"
	"time"
)

type Coordinator struct {
	config Config

	master *Node
	nodes  map[string]*Node

	healthchecks    map[string]*map[int]error
	healthcheckStat map[string]int
	lastHealthcheck int
}

var ErrNoHealthyCandidates = fmt.Errorf("no healthy candidates")

func NewCoordinator(config Config) (*Coordinator, error) {
	c := Coordinator{
		config: config,
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
	zap.S().Info("Coordinator start")
	c.loop(ctx)
	zap.S().Info("Coordinator exit")
}

func (c *Coordinator) loop(ctx context.Context) {
	ticker := time.NewTicker(50 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.master == nil {
				c.elect()
				continue
			}

			if !c.IsHealthy(c.master.name) {
				c.revokeCurrentMaster()
			}
		}
	}
}

func (c *Coordinator) revokeCurrentMaster() {
	zap.S().Warn("Revoke unhealthy master %s", c.master.name)

	var hash common.Hash
	if err := c.master.opNode.CallContext(context.Background(), &hash, "admin_stopSequencer"); err != nil {
		zap.S().Error("Fail to call admin_stopSequencer on %s even though its leadership will be revoked, error: %+v", c.master.name, err)
		c.master = nil
	} else {
		c.master = nil
	}
}

func (c *Coordinator) elect() {
	zap.S().Info("Start election")
	if existingMaster := c.findExistingMaster(); existingMaster != nil {
		c.master = existingMaster
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.config.MaxConvergenceWaitingTimeMs)*time.Millisecond)
	defer cancel()
	_ = c.waitForConvergence(ctx)

	canonical, canonicalStatus, err := c.findCanonicalCandidate()
	if canonical == nil {
		zap.S().Error("Fail to find canonical candidate, error: %+v", err)
		return
	}

	err = canonical.opNode.CallContext(context.Background(), nil, "admin_startSequencer", canonicalStatus.UnsafeL2.Hash)
	if err != nil {
		zap.S().Error("Fail to call admin_startSequencer on %s, error: %+v", canonical.name, err)
		return
	}

	c.master = canonical
	zap.S().Error("Success to elect new master %s, unsafe_l2: %v", canonical.name, canonicalStatus.UnsafeL2)
}

func (c *Coordinator) waitForConvergence(ctx context.Context) bool {
	zap.S().Info("Wait nodes to converge on the same height")

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

func (c *Coordinator) nodesConverged() bool {
	var channel = make(chan uint64, len(c.nodes))
	var wg sync.WaitGroup

	for nodeName, node := range c.nodes {
		if !c.IsHealthy(nodeName) {
			continue
		}

		wg.Add(1)
		go func(node *Node) {
			defer wg.Done()

			var syncStatus eth.SyncStatus
			if err := node.opNode.CallContext(context.Background(), &syncStatus, "optimism_syncStatus"); err != nil {
				zap.S().Errorf("Fail to call optimism_syncStatus on %s, error: %+v", node.name, err)
				return
			}

			channel <- syncStatus.UnsafeL2.Number
		}(node)
	}
	wg.Wait()

	var convergence uint64
	for unsafeL2 := range channel {
		if convergence == 0 {
			convergence = unsafeL2
		} else if convergence != unsafeL2 {
			return false
		}
	}
	return true
}

func (c *Coordinator) isCandidate(nodeName string) bool {
	return c.config.Candidates[nodeName] != nil
}

func (c *Coordinator) findCanonicalCandidate() (*Node, *eth.SyncStatus, error) {
	var canonical *Node
	var canonicalStatus *eth.SyncStatus
	for nodeName := range c.config.Candidates {
		// Filter healthy candidates
		if !c.isCandidate(nodeName) || !c.IsHealthy(nodeName) {
			continue
		}

		candidate := c.nodes[nodeName]
		var syncStatus eth.SyncStatus
		if err := candidate.opNode.CallContext(context.Background(), &syncStatus, "optimism_syncStatus"); err != nil {
			zap.S().Errorf("Fail to call optimism_syncStatus on %s, error: %+v", candidate.name, err)
			continue
		}

		if canonicalStatus == nil || canonicalStatus.UnsafeL2.Number < syncStatus.UnsafeL2.Number {
			canonical = candidate
			canonicalStatus = &syncStatus
		}
	}

	if canonical == nil {
		return nil, nil, ErrNoHealthyCandidates
	}
	return canonical, canonicalStatus, nil
}

func (c *Coordinator) findExistingMaster() *Node {
	for nodeName := range c.config.Candidates {
		// Filter healthy candidates
		if !c.isCandidate(nodeName) || !c.IsHealthy(nodeName) {
			continue
		}

		var sequencerStopped bool
		candidate := c.nodes[nodeName]
		err := candidate.opNode.CallContext(context.Background(), &sequencerStopped, "admin_sequencerStopped")
		if err != nil {
			continue
		}

		if sequencerStopped == false {
			return candidate
		}
	}

	return nil
}
