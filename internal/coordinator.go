package internal

import (
	"context"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"go.uber.org/zap"
	"sync"
	"time"
)

type Coordinator struct {
	config Config

	master     *Node
	candidates map[string]Node

	healthchecks    map[string]*map[int]error
	healthcheckStat map[string]int
	lastHealthcheck int
}

var ErrNoHealthyCandidates = fmt.Errorf("no healthy candidates")

func NewCoordinator(config Config) (*Coordinator, error) {
	c := Coordinator{
		config: config,
	}

	// Create clients for candidates
	for nodeName, nodeCfg := range config.Candidates {
		opNodeClient, err := rpc.Dial(nodeCfg.OpNodePublicRpcUrl)
		if err != nil {
			return nil, fmt.Errorf("dial OpNode error, nodeName: %s, OpNodeUrl: %s, error: %v", nodeName, nodeCfg.OpNodePublicRpcUrl, err)
		}
		opGethClient, err := dialEthClientWithTimeout(context.Background(), nodeCfg.OpGethPublicRpcUrl)
		if err != nil {
			return nil, fmt.Errorf("dial OpGeth error, nodeName: %s, OpGethUrl: %s, error: %v", nodeName, nodeCfg.OpGethPublicRpcUrl, err)
		}

		c.candidates[nodeName] = Node{
			name:   nodeName,
			opNode: opNodeClient,
			opGeth: opGethClient,
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

			if !c.IsHealthy(c.master) {
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

func (c *Coordinator) findCanonicalCandidate() (*Node, *eth.SyncStatus, error) {
	var canonical *Node
	var canonicalStatus *eth.SyncStatus
	for _, candidate := range c.candidates {
		if !c.IsHealthy(&candidate) {
			continue
		}

		var syncStatus eth.SyncStatus
		if err := candidate.opNode.CallContext(context.Background(), &syncStatus, "optimism_syncStatus"); err != nil {
			zap.S().Errorf("Fail to call optimism_syncStatus on %s, error: %+v", candidate.name, err)
			continue
		}

		if canonicalStatus == nil || canonicalStatus.UnsafeL2.Number < syncStatus.UnsafeL2.Number {
			canonical = &candidate
			canonicalStatus = &syncStatus
		}
	}

	if canonical == nil {
		return nil, nil, ErrNoHealthyCandidates
	}
	return canonical, canonicalStatus, nil
}

func (c *Coordinator) findExistingMaster() *Node {
	for _, candidate := range c.candidates {
		var sequencerStopped bool
		err := candidate.opNode.CallContext(context.Background(), &sequencerStopped, "admin_sequencerStopped")
		if err != nil {
			continue
		}

		if sequencerStopped == false {
			return &candidate
		}
	}

	return nil
}

func (c *Coordinator) waitForConvergence(ctx context.Context) bool {
	zap.S().Info("Wait nodes to converge on the same height")

	ticker := time.NewTicker(10 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			zap.S().Warn("Timeout waiting for candidates to converge on the same height")
			return false
		case <-ticker.C:
			if c.candidatesConverged() {
				zap.S().Infof("Candidates have converged on the same height")
				return true
			}
		}
	}
}

func (c *Coordinator) candidatesConverged() bool {
	var channel = make(chan uint64, len(c.candidates))
	var wg sync.WaitGroup

	for _, node := range c.candidates {
		if !c.IsHealthy(&node) {
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
		}(&node)
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
