package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/node-real/op-coordinator/core"
)

type EthAPI struct {
	election *core.Election
}

func NewEthAPI(election *core.Election) *EthAPI {
	return &EthAPI{election: election}
}

// GetBlockByNumber Returns information of the block matching the given block number from the _leader sequencer_.
//
// This API is similar to the RPC `eth_getBlockByNumber` in go-ethereum.
func (api *EthAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, _detail bool) (*json.RawMessage, error) {
	forwarder := api.election.Master()
	if forwarder == nil {
		return nil, errors.New("forwarder is unavailable")
	}

	var result *json.RawMessage
	err := forwarder.OpGethRPC.CallContext(ctx, &result, "eth_getBlockByNumber", number, true)
	if err != nil {
		return nil, err
	}

	return result, nil
}
