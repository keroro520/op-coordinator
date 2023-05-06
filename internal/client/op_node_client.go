package client

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/client"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/common"
)

type OpNodeClient struct {
	rpc client.RPC
}

func NewOpNodeClient(rpc client.RPC) *OpNodeClient {
	return &OpNodeClient{rpc}
}

func (r *OpNodeClient) SyncStatus(ctx context.Context) (*eth.SyncStatus, error) {
	var output *eth.SyncStatus
	err := r.rpc.CallContext(ctx, &output, "optimism_syncStatus")
	return output, err
}

func (r *OpNodeClient) SequencerStopped(ctx context.Context) (bool, error) {
	var output bool
	err := r.rpc.CallContext(ctx, &output, "admin_sequencerStopped")
	return output, err
}

func (r *OpNodeClient) StartSequencer(ctx context.Context, blockHash common.Hash) error {
	err := r.rpc.CallContext(ctx, nil, "admin_startSequencer", blockHash)
	return err
}

func (r *OpNodeClient) StopSequencer(ctx context.Context) (*common.Hash, error) {
	var output *common.Hash
	err := r.rpc.CallContext(ctx, &output, "admin_stopSequencer")
	return output, err
}
