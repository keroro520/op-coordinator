package client

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/client"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/common"
)

type RollupClient struct {
	rpc client.RPC
}

func NewRollupClient(rpc client.RPC) *RollupClient {
	return &RollupClient{rpc}
}

func (r *RollupClient) SyncStatus(ctx context.Context) (*eth.SyncStatus, error) {
	var output *eth.SyncStatus
	err := r.rpc.CallContext(ctx, &output, "optimism_syncStatus")
	return output, err
}

func (r *RollupClient) IsMaster(ctx context.Context) (bool, error) {
	var output bool
	err := r.rpc.CallContext(ctx, &output, "optimism_isMaster")
	return output, err
}

func (r *RollupClient) StartSequencer(ctx context.Context, blockHash common.Hash) error {
	var output *error
	err := r.rpc.CallContext(ctx, &output, "admin_startSequencer", blockHash)
	return err
}

func (r *RollupClient) StopSequencer(ctx context.Context) (common.Hash, error) {
	var output *common.Hash
	err := r.rpc.CallContext(ctx, &output, "admin_stopSequencer")
	return *output, err
}
