package internal

import "errors"

// AdminCommand is the interface for admin commands. Admin commands come from HTTP API and are executed by
// Coordinator synchronously.
type AdminCommand interface {
	Execute(coordinator *Coordinator)
	RespCh() chan error
}

// SetMasterCommand is used to set master node
type SetMasterCommand struct {
	nodeName string
	respCh   chan error
}

func NewSetMasterCommand(nodeName string) SetMasterCommand {
	return SetMasterCommand{
		nodeName: nodeName,
		respCh:   make(chan error),
	}
}

func (cmd SetMasterCommand) Execute(coordinator *Coordinator) {
	if !coordinator.isCandidate(cmd.nodeName) {
		cmd.RespCh() <- errors.New("node is not a candidate")
		return
	}
	if coordinator.master == cmd.nodeName {
		cmd.RespCh() <- errors.New("node is already the master")
		return
	}

	coordinator.revokeCurrentMaster()
	cmd.RespCh() <- coordinator.setMaster(cmd.nodeName)
}

func (cmd SetMasterCommand) RespCh() chan error {
	return cmd.respCh
}
