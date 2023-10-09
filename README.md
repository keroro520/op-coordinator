# Coordinator

## Description

Coordinator ensures that there will be one and only one instance of Optimism sequencer is producing blocks at any given time.

## Motivation

The Optimism sequencer plays a critical role in the production of blocks within the optimistic rollup. However, the current block production process lacks decentralization and is controlled by a single entity. Additionally, the existing implementation of the Optimism sequencer lacks a fail-over mechanism, making it a single point of failure. If the sequencer goes down, the entire optimistic rollup halts block production.

To address these issues, the Coordinator has been introduced. It ensures that only one instance of the Optimism sequencer is actively producing blocks, and in the event of a failure, it can be replaced by another instance.

Auto-Election is a feature that ensures only one instance of the Optimism sequencer is producing blocks at any given time.

## Spec

Optimism node maintains a state flag `sequencerActive` to indicate its right to produce blocks. Meanwhile, node provide a set of `admin_` HTTP API to query and control the state flag:
- `admin_sequencerActive` query whether the node have right to active produce blocks
- `admin_startSequencer` grant the node right to active produce blocks
- `admin_stopSequencer` revoke the node right to active produce blocks

So Coordinator control the right to active produce blocks via `admin_` API. It ensures that only one sequencer will peoduce blocks at any time, named as [active sequencer][active-sequencer], and act [fail-over] when the current [active sequencer][active-sequencer] is [fault detected][fault-detection]. The details will follow.


### Glossary

#### Sequencer

[sequencer]: README.md#sequencer

Refers the a node that runs with `--sequencer.enabled=true`.

#### Active Sequencer

[active-sequencer]: README.md#active-sequencer

A [sequencer] maintains a state flag `sequencerActive` to indicate its right to produce blocks. When the `sequencerActive` is `true`, we refer this [sequencer] as an [active sequencer][active-sequencer], otherwise [inactive sequencer][inactive-sequencer].

**Coordinator ensures that there will be at most 1 active sequencer at any time.**

#### Inactive Sequencer

[inactive-sequencer]: README.md#inactive-sequencer

See also [active-sequencer].

#### Stopped Hash

[stopped-hash]: README.md#stopped-hash

The hash returned by [active sequencer] when calling `admin_stopSequencer`.

### Fault Detection

[fault-detection]: README.md#fault-detection

Fault detection mechanism is used for judging whether a sequencer is healthy.

Coordinator periodically requests `optimism_syncStatus` and `eth_getBlockByNumber("latest")` to sequencers and predicates their health status. in addition, to eliminate false predication caused by network jitter, we choose latest 5 (configurable) responses, if the latest 5 responses are all errored, the corresponding sequencer is unhealthy, otherwise it's healthy.

### Fail-Over

[fail-over]: README.md#fail-over

When the current active sequencer becomes unhealthy, the fail-over mechanism was activated.

#### 1. Revoke Block-Production Right from Current Active Sequencer

(1) query the state flag `sequencerActive` to current [active sequencer][active-sequencer] via `admin_sequencerActive`.

(2) When the flag is true, call `admin_stopSequencer` to revoke block-production right and record the responded hash as [stopped hash][stopped-hash]; when the flag is false, call `optimism_syncStatus` and record the responsed `unsafe_l2.hash` as [stopped hash][stopped-hash].

Whenever errors occur, go back to (1) and retry.

#### 2. Elect A Candidate Sequencer

Elect a new sequencer as candidate [active sequencer][active-sequencer]. The candidate sequencer must:

1. Be healthy;
2. Response OK for calling `eth_getBlockByHash(stopped hash)`;
3. Maintain the highest chain number amount healthy sequencers.

#### 3. Grant Block-Production Right to The Candidate Sequencer

Grant block-production right to the candidate sequencer by calling `admin_startSequencer(stopped hash)`.

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
