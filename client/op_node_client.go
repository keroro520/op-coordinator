package client

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/client"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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

func (r *OpNodeClient) SequencerActive(ctx context.Context) (bool, error) {
	var active bool
	err := r.rpc.CallContext(ctx, &active, "admin_sequencerActive")
	if err != nil {
		var stopped bool
		err = r.rpc.CallContext(ctx, &stopped, "admin_sequencerStopped")
		return !stopped, err
	}
	return active, err
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

func (r *OpNodeClient) ResetDerivationPipeline(ctx context.Context) error {
	err := r.rpc.CallContext(ctx, nil, "admin_resetDerivationPipeline")
	return err
}

func (r *OpNodeClient) OutputAtBlock(ctx context.Context, blockNum uint64) (*eth.OutputResponse, error) {
	var output *eth.OutputResponse
	err := r.rpc.CallContext(ctx, &output, "optimism_outputAtBlock", hexutil.Uint64(blockNum))
	return output, err
}

func (r *OpNodeClient) RollupConfig(ctx context.Context) (*rollup.Config, error) {
	var output *rollup.Config
	err := r.rpc.CallContext(ctx, &output, "optimism_rollupConfig")
	return output, err
}

func (r *OpNodeClient) Version(ctx context.Context) (string, error) {
	var output string
	err := r.rpc.CallContext(ctx, &output, "optimism_version")
	return output, err
}
