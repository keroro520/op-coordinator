package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/node-real/op-coordinator/bridge"
	"github.com/node-real/op-coordinator/config"
)

type RollupAPI struct {
	config.ForwardConfig
	highestBridge *bridge.HighestBridge
}

func NewRollupAPI(cfg config.Config, highestBridge *bridge.HighestBridge) *RollupAPI {
	return &RollupAPI{ForwardConfig: cfg.Forward, highestBridge: highestBridge}
}

func (api *RollupAPI) SyncStatus(ctx context.Context) (*eth.SyncStatus, error) {
	highestBridge := api.highestBridge.Highest()
	if highestBridge == nil {
		return nil, errors.New("all bridges are unavailable")
	}

	syncStatus, err := highestBridge.OpNode.SyncStatus(ctx)
	if err != nil {
		return nil, err
	}

	if syncStatus.UnsafeL2.Number < uint64(api.SubSyncStatusUnsafeL2Number) {
		syncStatus.UnsafeL2.Number = 0
	} else {
		syncStatus.UnsafeL2.Number -= uint64(api.SubSyncStatusUnsafeL2Number)
	}
	return syncStatus, nil
}

func (api *RollupAPI) OutputAtBlock(ctx context.Context, blockNum hexutil.Uint64) (*json.RawMessage, error) {
	return api.callContext(ctx, "optimism_outputAtBlock", blockNum)
}

func (api *RollupAPI) RollupConfig(ctx context.Context) (*json.RawMessage, error) {
	return api.callContext(ctx, "optimism_rollupConfig")
}

func (api *RollupAPI) Version(ctx context.Context) (*json.RawMessage, error) {
	return api.callContext(ctx, "optimism_version")
}

func (api *RollupAPI) callContext(ctx context.Context, method string, args ...interface{}) (*json.RawMessage, error) {
	highestBridge := api.highestBridge.Highest()
	if highestBridge == nil {
		return nil, errors.New("all bridges are unavailable")
	}

	var result *json.RawMessage
	err := highestBridge.OpNodeRPC.CallContext(ctx, &result, method, args...)
	if err != nil {
		return nil, err
	}

	return result, nil
}
