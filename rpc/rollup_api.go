package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/node-real/op-coordinator/config"
	"github.com/node-real/op-coordinator/core"
)

type RollupAPI struct {
	config.ForwardConfig
	election *core.Election
}

func NewRollupAPI(cfg config.Config, e *core.Election) *RollupAPI {
	return &RollupAPI{ForwardConfig: cfg.Forward, election: e}
}

func (api *RollupAPI) SyncStatus(ctx context.Context) (*eth.SyncStatus, error) {
	forwarder := api.election.Master()
	if forwarder == nil {
		return nil, errors.New("forwarder is unavailable")
	}

	syncStatus, err := forwarder.OpNode.SyncStatus(ctx)
	if err != nil {
		return nil, err
	}

	if syncStatus.UnsafeL2.Number < uint64(api.SubSyncStatusUnsafeL2Number) {
		syncStatus.UnsafeL2.Number = 0
	} else {
		syncStatus.UnsafeL2.Number -= uint64(api.SubSyncStatusUnsafeL2Number)
	}
	return syncStatus, nil
}

func (api *RollupAPI) OutputAtBlock(ctx context.Context, blockNum hexutil.Uint64) (*json.RawMessage, error) {
	return api.callContext(ctx, "optimism_outputAtBlock", blockNum)
}

func (api *RollupAPI) RollupConfig(ctx context.Context) (*json.RawMessage, error) {
	return api.callContext(ctx, "optimism_rollupConfig")
}

func (api *RollupAPI) Version(ctx context.Context) (*json.RawMessage, error) {
	return api.callContext(ctx, "optimism_version")
}

func (api *RollupAPI) callContext(ctx context.Context, method string, args ...interface{}) (*json.RawMessage, error) {
	forwarder := api.election.Master()
	if forwarder == nil {
		return nil, errors.New("forwarder is unavailable")
	}

	var result *json.RawMessage
	err := forwarder.OpNodeRPC.CallContext(ctx, &result, method, args...)
	if err != nil {
		return nil, err
	}

	return result, nil
}
