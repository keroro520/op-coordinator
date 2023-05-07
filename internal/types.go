package internal

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/node-real/op-coordinator/internal/client"
)

type Node struct {
	name   string
	opNode *client.OpNodeClient
	opGeth *ethclient.Client
}

func NewNode(nodeName string, opNodeUrl string, opGethUrl string) (*Node, error) {
	opNodeClient, err := dialOpNodeClientWithTimeout(context.Background(), opNodeUrl)
	if err != nil {
		return nil, fmt.Errorf("dial OpNode error, nodeName: %s, OpNodeUrl: %s, error: %v", nodeName, opNodeUrl, err)
	}
	opGethClient, err := dialEthClientWithTimeout(context.Background(), opGethUrl)
	if err != nil {
		return nil, fmt.Errorf("dial OpGeth error, nodeName: %s, OpGethUrl: %s, error: %v", nodeName, opGethUrl, err)
	}

	return &Node{
		name:   nodeName,
		opNode: opNodeClient,
		opGeth: opGethClient,
	}, nil
}
