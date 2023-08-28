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
// design, the master node is the only node that can request a building block. If the master node is not the node
// that calls this function, the function returns an error. In another word, RequestBuildingBlock ensures that
// only the master node will build new blocks, so that we enforce the data consistency.
//
// Note that the `nodeName` parameter should be identical to the node name in the configuration file.
func (api *ElectionAPI) RequestBuildingBlock(nodeName string) error {
	master := api.election.Master()
	if master == nil {
		return fmt.Errorf("empty master")
	}

	if master.Name != nodeName {
		go func() {
			api.log.Warn("Invalid master is requesting!", "master", master.Name, "node", nodeName)

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
		return fmt.Errorf("unknown master")
	}
	api.log.Debug("Allow to produce blocks", "node", nodeName)
	return nil
}

// GetMaster returns the current master node name.
func (api *ElectionAPI) GetMaster() string {
	if master := api.election.Master(); master == nil {
		return ""
	} else {
		return master.Name
	}
}

// SetMaster sets the master node name manually.
func (api *ElectionAPI) SetMaster(nodeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	return executeAdminCommand(ctx, api.election, core.NewSetMasterCommand(nodeName))
}

// StartElection enables the auto-detection and auto-election process. See StopElection for more details.
func (api *ElectionAPI) StartElection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	return executeAdminCommand(ctx, api.election, core.NewStartElectionCommand())
}

// StopElection disables the auto-detection and auto-election process. When the election is stopped, the master node
// will not be changed even if the current master node is down. The election process can be started again by calling
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
