package rpc

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/node-real/op-coordinator/core"
	"time"
)

// ElectionAPI is the API for the coordinator.
type ElectionAPI struct {
	log      log.Logger
	version  string
	election *core.Election
}

// NewElectionAPI creates a new ElectionAPI instance.
func NewElectionAPI(version string, e *core.Election, log log.Logger) *ElectionAPI {
	return &ElectionAPI{log: log, version: version, election: e}
}

func (api *ElectionAPI) Version() (string, error) {
	return api.version, nil
}

// RequestBuildingBlock is called by the sequencer to request a building block. According to the high-availability
// design, the active sequencer is the only node that can request a building block. If the active sequencer is not the node
// that calls this function, the function returns an error. In another word, RequestBuildingBlock ensures that
// only the active sequencer will build new blocks, so that we enforce the data consistency.
//
// Note that the `nodeName` parameter should be identical to the node name in the configuration file.
func (api *ElectionAPI) RequestBuildingBlock(nodeName string) error {
	activeSequencer := api.election.ActiveSequencer()
	if activeSequencer == nil {
		return fmt.Errorf("empty active sequencer")
	}

	if activeSequencer.Name != nodeName {
		go func() {
			api.log.Warn("Unmatched active sequencer is requesting!", "active sequencer", activeSequencer.Name, "node", nodeName)

			node := api.election.Nodes[nodeName]
			if node != nil && node.OpNode != nil {
				blockHash, err := node.OpNode.StopSequencer(context.Background())
				if err != nil {
					api.log.Error("Call admin_stopSequencer when RequestBuildingBlock", "node", nodeName, "error", err)
				} else {
					api.log.Info("Call admin_stopSequencer", "node", nodeName, "blockHash", blockHash.String())
				}
			}
		}()
		return fmt.Errorf("unmatched active sequencer")
	}
	api.log.Debug("Allow to produce blocks", "node", nodeName)
	return nil
}

// GetActiveSequencer returns the current active sequencer name.
func (api *ElectionAPI) GetActiveSequencer() string {
	return api.election.ActiveSequencerName()
}

// GetMaster returns the current active sequencer name.
// Deprecated, use GetActiveSequencer instead
func (api *ElectionAPI) GetMaster() string {
	return api.GetActiveSequencer()
}

// SetActiveSequencer sets the active sequencer name manually.
func (api *ElectionAPI) SetActiveSequencer(nodeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	return executeAdminCommand(ctx, api.election, core.NewSetActiveSequencerCommand(nodeName))
}

// SetMaster sets the active sequencer name manually.
// Deprecated, use SetActiveSequencer instead
func (api *ElectionAPI) SetMaster(nodeName string) error {
	return api.SetActiveSequencer(nodeName)
}

// StartElection enables the auto-detection and auto-election process. See StopElection for more details.
func (api *ElectionAPI) StartElection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	return executeAdminCommand(ctx, api.election, core.NewStartElectionCommand())
}

// StopElection disables the auto-detection and auto-election process. When the election is stopped, the active sequencer
// will not be changed even if the current active sequencer is down. The election process can be started again by calling
// StartElection.
//
// This API is used for debugging purpose and handling accidental situations.
func (api *ElectionAPI) StopElection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	return executeAdminCommand(ctx, api.election, core.NewStopElectionCommand())
}

// ElectionStopped returns true if the election process is stopped.
func (api *ElectionAPI) ElectionStopped() bool {
	return api.election.Config.Election.Stopped
}

func (api *ElectionAPI) GetStoppedHash(_ context.Context) (*common.Hash, error) {
	return api.election.StoppedHash(), nil
}

// executeAdminCommand executes an admin command and returns the result.
func executeAdminCommand(ctx context.Context, election *core.Election, cmd core.AdminCommand) error {
	election.AdminCh() <- cmd

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-cmd.RespCh():
		return err
	}
}
