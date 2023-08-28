package core

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/node-real/op-coordinator/types"
)

func FindCanonicalNode(nodes []*types.Node) (*types.Node, error) {
	var canonical *types.Node
	var canonicalStatus *eth.SyncStatus
	for _, node := range nodes {
		syncStatus, err := node.OpNode.SyncStatus(context.Background())
		if err == nil && (canonicalStatus == nil || canonicalStatus.UnsafeL2.Number < syncStatus.UnsafeL2.Number) {
			canonical = node
			canonicalStatus = syncStatus
		}
	}

	if canonical == nil {
		return nil, errors.New("no healthy node")
	}
	return canonical, nil
}

func FindActiveNodes(nodes []*types.Node) ([]*types.Node, error) {
	actives := make([]*types.Node, 0)
	for _, node := range nodes {
		active, err := node.OpNode.SequencerActive(context.Background())
		if err != nil {
			return nil, err
		} else if active {
			actives = append(actives, node)
		}
	}
	return actives, nil
}

// NodesConverged returns true if all the nodes have the same unsafe_l2 height
func NodesConverged(nodes []*types.Node) bool {
	var convergence uint64 = 0
	var convergenceFlag = true
	var convergenceLock sync.Mutex
	var wg sync.WaitGroup

	wg.Add(len(nodes))
	for _, node := range nodes {
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

func WaitForNodesConvergence(log log.Logger, ctx context.Context, nodes []*types.Node) bool {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		log.Info("Wait nodes to converge on the same height", "elapsed", duration)
	}()

	ticker := time.NewTicker(10 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			log.Warn("Timeout waiting for nodes to converge on the same height")
			return false
		case <-ticker.C:
			if NodesConverged(nodes) {
				log.Info("Nodes have converged on the same height successfully")
				return true
			}
		}
	}
}

func bool2float64(b bool) float64 {
	const FALSE = float64(0)
	const TRUE = float64(1)
	if b {
		return TRUE
	}
	return FALSE
}
