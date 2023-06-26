package types

import (
	"context"
	"fmt"
	client2 "github.com/ethereum-optimism/optimism/op-node/client"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/node-real/op-coordinator/client"
	"time"
)

type Node struct {
	Name      string
	OpNode    *client.OpNodeClient
	OpNodeRPC *rpc.Client
	OpGeth    *ethclient.Client
	OpGethRPC *rpc.Client
}

func NewNode(nodeName string, opNodeUrl string, opGethUrl string) (*Node, error) {
	opNode, opNodeRPC, err := dialOpNodeClientWithTimeout(context.Background(), opNodeUrl)
	if err != nil {
		return nil, fmt.Errorf("dial OpNode error, nodeName: %s, OpNodeUrl: %s, error: %v", nodeName, opNodeUrl, err)
	}
	opGeth, opGethRPC, err := dialEthClientWithTimeout(context.Background(), opGethUrl)
	if err != nil {
		return nil, fmt.Errorf("dial OpGeth error, nodeName: %s, OpGethUrl: %s, error: %v", nodeName, opGethUrl, err)
	}

	return &Node{
		Name:      nodeName,
		OpNode:    opNode,
		OpGeth:    opGeth,
		OpNodeRPC: opNodeRPC,
		OpGethRPC: opGethRPC,
	}, nil
}

const (
	// defaultDialTimeout is default duration the service will wait on
	// startup to make a connection to either the L1 or L2 backends.
	defaultDialTimeout = 5 * time.Second
)

// dialEthClientWithTimeout attempts to dial the L1 provider using the provided
// URL. If the dial doesn't complete within defaultDialTimeout seconds, this
// method will return an error.
func dialEthClientWithTimeout(ctx context.Context, url string) (*ethclient.Client, *rpc.Client, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()

	c, err := rpc.DialContext(ctx, url)
	if err != nil {
		return nil, nil, err
	}

	return ethclient.NewClient(c), c, nil
}

// dialOpNodeClientWithTimeout attempts to dial the RPC provider using the provided
// URL. If the dial doesn't complete within defaultDialTimeout seconds, this
// method will return an error.
func dialOpNodeClientWithTimeout(ctx context.Context, url string) (*client.OpNodeClient, *rpc.Client, error) {
	ctxt, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()

	rpcCl, err := rpc.DialContext(ctxt, url)
	if err != nil {
		return nil, nil, err
	}

	return client.NewOpNodeClient(client2.NewBaseRPCClient(rpcCl)), rpcCl, nil
}
