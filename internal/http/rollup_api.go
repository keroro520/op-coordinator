package http

import (
	"context"
	"errors"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
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

func (api *RollupAPI) OutputAtBlock(ctx context.Context, blockNum uint64) (*eth.OutputResponse, error) {
	master := api.coordinator.Nodes[api.coordinator.Master]
	if master == nil {
		return nil, errors.New("empty master")
	}
	return master.OpNode.OutputAtBlock(ctx, blockNum)
}

func (api *RollupAPI) RollupConfig(ctx context.Context) (*rollup.Config, error) {
	master := api.coordinator.Nodes[api.coordinator.Master]
	if master == nil {
		return nil, errors.New("empty master")
	}
	return master.OpNode.RollupConfig(ctx)
}

func (api *RollupAPI) Version(ctx context.Context) (string, error) {
	master := api.coordinator.Nodes[api.coordinator.Master]
	if master == nil {
		return "", errors.New("empty master")
	}
	return master.OpNode.Version(ctx)
}
