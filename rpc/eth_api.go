package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/node-real/op-coordinator/bridge"
)

type EthAPI struct {
	highestBridge *bridge.HighestBridge
}

func NewEthAPI(highestBridge *bridge.HighestBridge) *EthAPI {
	return &EthAPI{highestBridge: highestBridge}
}

// GetBlockByNumber Returns information of the block matching the given block number from the highest bridge node.
//
// This API is similar to the RPC `eth_getBlockByNumber` in go-ethereum.
func (api *EthAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, _detail bool) (*json.RawMessage, error) {
	highestBridge := api.highestBridge.Highest()
	if highestBridge == nil {
		return nil, errors.New("all bridges are unavailable")
	}

	var result *json.RawMessage
	err := highestBridge.OpGethRPC.CallContext(ctx, &result, "eth_getBlockByNumber", number, true)
	if err != nil {
		return nil, err
	}

	return result, nil
}
