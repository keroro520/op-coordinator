package core

import (
	"context"
	"errors"
	"time"
)

// AdminCommand is the interface for admin commands. Admin commands come from HTTP API and are executed by
// Election synchronously.
type AdminCommand interface {
	Execute(election *Election)
	RespCh() chan error
}

// SetActiveSequencerCommand is used to set the active sequencer
type SetActiveSequencerCommand struct {
	nodeName string
	respCh   chan error
}

type StartElectionCommand struct {
	respCh chan error
}

type StopElectionCommand struct {
	respCh chan error
}

func NewSetActiveSequencerCommand(nodeName string) SetActiveSequencerCommand {
	return SetActiveSequencerCommand{
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

func (cmd SetActiveSequencerCommand) Execute(election *Election) {
	if !election.IsSequencer(cmd.nodeName) {
		cmd.RespCh() <- errors.New("node is not a sequencer")
		return
	}
	if election.ActiveSequencer() != nil && election.ActiveSequencer().Name == cmd.nodeName {
		cmd.RespCh() <- errors.New("node is already the active sequencer")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(election.Config.Election.MaxWaitingTimeForConvergenceMs)*time.Millisecond)
	defer cancel()
	WaitForNodesConvergence(election.log, ctx, election.HealthySequencers())

	err := election.RevokeActiveSequencer()
	if err != nil {
		cmd.RespCh() <- err
		return
	}

	err = election.SetActiveSequencer(cmd.nodeName)
	cmd.RespCh() <- err
}

func (cmd SetActiveSequencerCommand) RespCh() chan error {
	return cmd.respCh
}

func (cmd StartElectionCommand) Execute(election *Election) {
	election.Config.Election.Stopped = false
	cmd.RespCh() <- nil
}

func (cmd StartElectionCommand) RespCh() chan error {
	return cmd.respCh
}

func (cmd StopElectionCommand) Execute(election *Election) {
	election.Config.Election.Stopped = true
	cmd.RespCh() <- nil
}

func (cmd StopElectionCommand) RespCh() chan error {
	return cmd.respCh
}
