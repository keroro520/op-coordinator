package http

import (
	"context"
	"errors"
	"fmt"
	"github.com/node-real/op-coordinator/internal/bridge"
	"github.com/node-real/op-coordinator/internal/coordinator"
	"go.uber.org/zap"
	"time"
)

// CoordinatorAPI is the API for the coordinator.
type CoordinatorAPI struct {
	version       string
	coordinator   *coordinator.Coordinator
	highestBridge *bridge.HighestBridge
}

// NewCoordinatorAPI creates a new CoordinatorAPI instance.
func NewCoordinatorAPI(version string, c *coordinator.Coordinator, h *bridge.HighestBridge) *CoordinatorAPI {
	return &CoordinatorAPI{version: version, coordinator: c, highestBridge: h}
}

func (api *CoordinatorAPI) Version() (string, error) {
	return api.version, nil
}

// RequestBuildingBlock is called by the sequencer to request a building block. According to the high-availability
// design, the master node is the only node that can request a building block. If the master node is not the node
// that calls this function, the function returns an error. In another word, RequestBuildingBlock ensures that
// only the master node will build new blocks, so that we enforce the data consistency.
//
// Note that the `nodeName` parameter should be identical to the node name in the configuration file.
func (api *CoordinatorAPI) RequestBuildingBlock(nodeName string) error {
	if api.coordinator.Master == "" {
		return fmt.Errorf("empty master")
	}

	if api.coordinator.Master != nodeName {
		go func() {
			zap.S().Warnw("Invalid master is requesting!", "master", api.coordinator.Master, "node", nodeName)

			node := api.coordinator.Nodes[nodeName]
			if node != nil && node.OpNode != nil {
				blockHash, err := node.OpNode.StopSequencer(context.Background())
				if err != nil {
					zap.S().Errorw("Fail to call admin_stopSequencer", "node", nodeName, "error", err)
				} else {
					zap.S().Infow("Success to call admin_stopSequencer", "blockHash", blockHash.String())
				}
			}
		}()
		return fmt.Errorf("unknown master")
	}

	return nil
}

// GetMaster returns the current master node name.
func (api *CoordinatorAPI) GetMaster() string {
	return api.coordinator.Master
}

// SetMaster sets the master node name manually.
func (api *CoordinatorAPI) SetMaster(nodeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	return executeAdminCommand(ctx, api.coordinator, coordinator.NewSetMasterCommand(nodeName))
}

// StartElection enables the auto-detection and auto-election process. See StopElection for more details.
func (api *CoordinatorAPI) StartElection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	return executeAdminCommand(ctx, api.coordinator, coordinator.NewStartElectionCommand())
}

// StopElection disables the auto-detection and auto-election process. When the election is stopped, the master node
// will not be changed even if the current master node is down. The election process can be started again by calling
// StartElection.
//
// This API is used for debugging purpose and handling accidental situations.
func (api *CoordinatorAPI) StopElection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	return executeAdminCommand(ctx, api.coordinator, coordinator.NewStopElectionCommand())
}

// ElectionStopped returns true if the election process is stopped.
func (api *CoordinatorAPI) ElectionStopped() bool {
	return api.coordinator.Config.Election.Stopped
}

func (api *CoordinatorAPI) SetHighestBridge(_ context.Context, opNodeUrl string, opGethUrl string) error {
	return api.highestBridge.SetHighest(opNodeUrl, opGethUrl)
}

func (api *CoordinatorAPI) GetHighestBridge(_ context.Context) (string, error) {
	if api.highestBridge == nil {
		return "", errors.New("HighestBridge is empty")
	}
	node := api.highestBridge.Highest()
	if node == nil {
		return "", errors.New("HighestBridge is empty")
	}
	return node.Name, nil
}

func (api *CoordinatorAPI) UnsetHighestBridge(_ context.Context) error {
	api.highestBridge.UnsetHighest()
	return nil
}

// executeAdminCommand executes an admin command and returns the result.
func executeAdminCommand(ctx context.Context, coordinator *coordinator.Coordinator, cmd coordinator.AdminCommand) error {
	coordinator.AdminCh() <- cmd

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-cmd.RespCh():
		return err
	}
}
