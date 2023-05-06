package internal

import (
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type Node struct {
	name   string
	opNode *rpc.Client
	opGeth *ethclient.Client
}
