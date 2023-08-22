package core

import (
	"context"
	"errors"
	"time"
)

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

type StartElectionCommand struct {
	respCh chan error
}

type StopElectionCommand struct {
	respCh chan error
}

func NewSetMasterCommand(nodeName string) SetMasterCommand {
	return SetMasterCommand{
		nodeName: nodeName,
		respCh:   make(chan error),
	}
}

func NewStartElectionCommand() StartElectionCommand {
	return StartElectionCommand{
		respCh: make(chan error),
	}
}

func NewStopElectionCommand() StopElectionCommand {
	return StopElectionCommand{
		respCh: make(chan error),
	}
}

func (cmd SetMasterCommand) Execute(coordinator *Coordinator) {
	if !coordinator.IsCandidate(cmd.nodeName) {
		cmd.RespCh() <- errors.New("node is not a candidate")
		return
	}
	if coordinator.Master == cmd.nodeName {
		cmd.RespCh() <- errors.New("node is already the master")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(coordinator.Config.Election.MaxWaitingTimeForConvergenceMs)*time.Millisecond)
	defer cancel()
	WaitForNodesConvergence(coordinator.log, ctx, coordinator.HealthyCandidates())

	err := coordinator.RevokeMaster()
	if err != nil {
		cmd.RespCh() <- err
		return
	}

	err = coordinator.setMaster(cmd.nodeName)
	cmd.RespCh() <- err
}

func (cmd SetMasterCommand) RespCh() chan error {
	return cmd.respCh
}

func (cmd StartElectionCommand) Execute(coordinator *Coordinator) {
	coordinator.Config.Election.Stopped = false
	cmd.RespCh() <- nil
}

func (cmd StartElectionCommand) RespCh() chan error {
	return cmd.respCh
}

func (cmd StopElectionCommand) Execute(coordinator *Coordinator) {
	coordinator.Config.Election.Stopped = true
	cmd.RespCh() <- nil
}

func (cmd StopElectionCommand) RespCh() chan error {
	return cmd.respCh
}
