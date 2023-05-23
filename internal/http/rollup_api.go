package http

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/node-real/op-coordinator/internal/config"
	"github.com/node-real/op-coordinator/internal/coordinator"
)

type RollupAPI struct {
	config.ForwardConfig
	coordinator *coordinator.Coordinator
}

func NewRollupAPI(cfg config.Config, c *coordinator.Coordinator) *RollupAPI {
	return &RollupAPI{ForwardConfig: cfg.Forward, coordinator: c}
}

func (api *RollupAPI) SyncStatus(ctx context.Context) (*eth.SyncStatus, error) {
	master := api.coordinator.Nodes[api.coordinator.Master]
	if master == nil {
		return nil, errors.New("empty master")
	}

	syncStatus, err := master.OpNode.SyncStatus(ctx)
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
	master := api.coordinator.Nodes[api.coordinator.Master]
	if master == nil {
		return nil, errors.New("empty master")
	}

	var result *json.RawMessage
	err := master.OpNodeRPC.CallContext(ctx, &result, method, args...)
	if err != nil {
		return nil, err
	}

	return result, nil
}
