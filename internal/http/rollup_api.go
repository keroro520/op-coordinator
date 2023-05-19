package http

import (
	"context"
	"errors"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/node-real/op-coordinator/internal/coordinator"
)

type RollupAPI struct {
	coordinator *coordinator.Coordinator
}

func NewRollupAPI(c *coordinator.Coordinator) *RollupAPI {
	return &RollupAPI{coordinator: c}
}

func (api *RollupAPI) SyncStatus(ctx context.Context) (*eth.SyncStatus, error) {
	master := api.coordinator.Nodes[api.coordinator.Master]
	if master == nil {
		return nil, errors.New("empty master")
	}

	return master.OpNode.SyncStatus(ctx)
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
