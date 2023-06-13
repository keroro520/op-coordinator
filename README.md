# OpCoordinator

## Description

OpCoordinator ensures that only one instance of Optimism sequencer is producing blocks at any given time.

## Motivation

Optimism sequencer is a single point of failure. If it goes down, the whole optimistic rollup stops producing blocks.
OpCoordinator comes to prevent this from happening, when the sequencer goes down, OpCoordinator can be used to switch
to a backup sequencer.

## Design

1. Kinds of nodes. There are 3 kinds of OP nodes: sequencers, bridges and rpcs. Sequencers are the nodes that produces blocks, bridges
are the intermediate nodes that are used to propagate blocks&transactions between sequencers and rpc nodes, rpc nodes
are the nodes that are used by users to interact with the optimistic rollup.
2. Transactions propagation. Users send transactions to rpc nodes, and then the transactions will propagate to all the other nodes via P2P network.
3. Blocks propagation. Sequencers will produce blocks and propagate them to all the other nodes via P2P network.
4. Blocks production. Only one sequencer is allowed to produce blocks at any given time. This is ensured by OpCoordinator. A sequencer
labeled with `optimism_sequencerStarted=true` requests permission to produce blocks from OpCoordinator before it starts
producing blocks. And OpCoordinator approves the request if the requesting sequencer is matched with the current
sequencer(recorded inside OpCoordinator memory). As OpCoordinator will record only one sequencer at any given time, we
can ensure that only one sequencer is producing blocks at any given time.
5. Handover. OpCoordinator will periodically check the health of all the nodes. If the current sequencer is down,
OpCoordinator will switch to a healthy backup sequencer.

## Usage

### Installation

```bash
go install github.com/node-real/op-coordinator
```

### Configuration

TODO

### Running

```bash
op-coordinator start --config config.yaml
```