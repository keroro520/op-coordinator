package http

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/node-real/op-coordinator/internal/bridge"
)

type EthAPI struct {
	highestBridge *bridge.HighestBridge
}

func NewEthAPI(highestBridge *bridge.HighestBridge) *EthAPI {
	return &EthAPI{highestBridge: highestBridge}
}

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
