package bridge

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/node-real/op-coordinator/config"
	"github.com/node-real/op-coordinator/types"
	"go.uber.org/zap"
	"time"
)

type HighestBridge struct {
	Config config.Config
	Nodes  map[string]*types.Node

	highest           string
	customizedHighest *types.Node
}

func NewHighestBridge(config config.Config) (*HighestBridge, error) {
	h := HighestBridge{
		Config: config,
		Nodes:  make(map[string]*types.Node),
	}

	// Create clients for nodes
	var err error
	for nodeName, nodeCfg := range config.Bridges {
		h.Nodes[nodeName], err = types.NewNode(nodeName, nodeCfg.OpNodePublicRpcUrl, nodeCfg.OpGethPublicRpcUrl)
		if err != nil {
			return nil, err
		}
	}

	return &h, nil
}

func (h *HighestBridge) Start(ctx context.Context) {
	lastWarningTime := time.Now()
	ticker := time.NewTicker(20 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var highestSyncStatus *eth.SyncStatus = nil
			for nodeName, node := range h.Nodes {
				syncStatus, err := node.OpNode.SyncStatus(ctx)
				if err != nil {
					continue
				}

				if highestSyncStatus == nil || highestSyncStatus.UnsafeL2.Number < syncStatus.UnsafeL2.Number {
					if h.highest != nodeName {
						zap.S().Warnf("Switch highest bridge, new is %v", nodeName)
					}
					h.highest = nodeName
					highestSyncStatus = syncStatus
				}
			}

			if h.highest == "" && lastWarningTime.Add(10*time.Second).After(time.Now()) {
				lastWarningTime = time.Now()
				zap.S().Warn("all bridges are unavailable")
			}
		}
	}
}

func (h *HighestBridge) Highest() *types.Node {
	if h.customizedHighest != nil {
		return h.customizedHighest
	}
	if node, ok := h.Nodes[h.highest]; ok {
		return node
	} else {
		return nil
	}
}

// TODO: rename

func (h *HighestBridge) SetHighest(opNodeUrl string, opGethUrl string) error {
	zap.S().Infow("set highest bridge", "opNodeUrl", opNodeUrl, "opGethUrl", opGethUrl)

	node, err := types.NewNode("setHighest", opNodeUrl, opGethUrl)
	if err != nil {
		return err
	}

	h.customizedHighest = node
	return nil
}

func (h *HighestBridge) UnsetHighest() {
	h.customizedHighest = nil
}
