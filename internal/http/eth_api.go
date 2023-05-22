package http

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/node-real/op-coordinator/internal/coordinator"
)

type EthAPI struct {
	coordinator *coordinator.Coordinator
}

func NewEthAPI(c *coordinator.Coordinator) *EthAPI {
	return &EthAPI{coordinator: c}
}

func (api *EthAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, _detail bool) (*json.RawMessage, error) {
	master := api.coordinator.Nodes[api.coordinator.Master]
	if master == nil {
		return nil, errors.New("empty master")
	}

	var result *json.RawMessage
	err := master.OpGethRPC.CallContext(ctx, &result, "eth_getBlockByNumber", number, true)
	if err != nil {
		return nil, err
	}

	return result, nil
}
