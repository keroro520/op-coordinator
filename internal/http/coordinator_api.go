package http

import (
	"context"
	"fmt"
	"github.com/node-real/op-coordinator/internal/coordinator"
	"go.uber.org/zap"
	"time"
)

type CoordinatorAPI struct {
	version     string
	coordinator *coordinator.Coordinator
}

func NewCoordinatorAPI(version string, c *coordinator.Coordinator) *CoordinatorAPI {
	return &CoordinatorAPI{version: version, coordinator: c}
}

func (api *CoordinatorAPI) Version() (string, error) {
	return api.version, nil
}

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

func (api *CoordinatorAPI) GetMaster() string {
	return api.coordinator.Master
}

func (api *CoordinatorAPI) SetMaster(nodeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return executeAdminCommand(ctx, api.coordinator, coordinator.NewSetMasterCommand(nodeName))
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
