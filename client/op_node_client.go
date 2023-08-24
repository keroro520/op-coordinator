package client

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/client"
	"github.com/ethereum-optimism/optimism/op-node/sources"
)

type OpNodeClient struct {
	*sources.RollupClient
	rpc client.RPC
}

// type OpNodeClient = sources.RollupClient

func NewOpNodeClient(rpc client.RPC) *OpNodeClient {
	return &OpNodeClient{
		RollupClient: sources.NewRollupClient(rpc),
		rpc:          rpc,
	}
}

func (r *OpNodeClient) ResetDerivationPipeline(ctx context.Context) error {
	err := r.rpc.CallContext(ctx, nil, "admin_resetDerivationPipeline")
	return err
}
