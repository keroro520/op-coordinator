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

### Running

```bash
op-coordinator start --config config.yaml
```

## RPC

### `coordinator` Namespace

#### `coordinator_getMaster`

Returns the current master node name.

RPC:

```
{"method": "coordinator_getMaster", "params": []}
```

#### `coordinator_setMaster`

Sets the master node name manually.

RPC:

```
{"method": "coordinator_setMaster", "params": [nodeName]}
```

- `nodeName`: A string represents the node name that will be set as the master node.

#### `coordinator_startElection`

Enables the auto-detection and auto-election process. See StopElection for more details.

RPC:

```
{"method": "coordinator_startElection", "params": []}
```

#### `coordinator_stopElection`

Disables the auto-detection and auto-election process. When the election is stopped, the master node
will not be changed even if the current master node is down. The election process can be started again by calling
StartElection.

This API is used for debugging purpose and handling accidental situations.

RPC:

```
{"method": "coordinator_stopElection", "params": []}
```

#### `coordinator_electionStopped`

Returns whether the auto-election process is stopped.

RPC:

```
{"method": "coordinator_electionStopped", "params": []}
```

#### `coordinator_requestBuildingBlock`

Returns whether the requesting node is allowed to build a block.

RequestBuildingBlock is called by the sequencer to request a building block. According to the high-availability
design, the master node is the only node that can request a building block. If the master node is not the node
that calls this function, the function returns an error. In another word, RequestBuildingBlock ensures that
only the master node will build new blocks, so that we enforce the data consistency.

Note that the `nodeName` parameter should be identical to the node name in the configuration file.

RPC:

```
{"method": "coordinator_requestBuildingBlock", "params": [nodeName]}
```

- `nodeName`: the node name that requests to build a block.

### `optimism` Namespace

#### `optimism_syncStatus`

See https://community.optimism.io/docs/developers/build/json-rpc/#eth-getblockrange

#### `optimism_outputAtBlock`

See https://community.optimism.io/docs/developers/build/json-rpc/#optimism-outputatblock

#### `optimism_rollupConfig`

See https://community.optimism.io/docs/developers/build/json-rpc/#optimism-rollupconfig

#### `optimism_version`

See https://community.optimism.io/docs/developers/build/json-rpc/#optimism-version

### `eth` Namespace

#### `eth_getBlockByNumber`

Returns information of the block matching the given block number.

RPC:

```
{"method": "eth_getBlockByNumber", "params": [blockTag, detail]}
```

- `blockTag`: The block number in hexadecimal format or the string latest, earliest, pending, safe or finalized.
- `detail`: The method returns the full transaction objects when this value is true otherwise, it returns only the hashes of the transactions.
