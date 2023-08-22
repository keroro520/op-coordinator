# Auto-Election

## Motivation

The Optimism sequencer plays a critical role in the production of blocks within the optimistic rollup. However, the current block production process lacks decentralization and is controlled by a single entity. Additionally, the existing implementation of the Optimism sequencer lacks a fail-over mechanism, making it a single point of failure. If the sequencer goes down, the entire optimistic rollup halts block production.

To address these issues, the Coordinator has been introduced. It ensures that only one instance of the Optimism sequencer is actively producing blocks, and in the event of a failure, it can be replaced by another instance.

Auto-Election is a feature that ensures only one instance of the Optimism sequencer is producing blocks at any given time.

## Glossary

- _sequencer_: OP Nodes configured with `--sequencer.enabled=true`
- _active sequencer_: The _sequencer_ node that maintains the `admin_sequencerActive` flag to indicate its right to produce blocks. When this flag is `true`, we refer to it as an _active sequencer_.
- _leader sequencer_: The _coordinator_ process maintains a `leader` variable, which points to the current _active sequencer_.
_ _follower sequencers_: The _sequencer_ nodes do not include _leader sequencer_.
- _stopped hash_: The hash returned by _leader sequencer_ when calling `admin_stopSequencer`

## Health Check

The _Coordinator_ performs periodic health checks on the OP Nodes and OP Geth by monitoring the `leader` variable. If a failure is detected or the `leader` is empty (`nil`), a fail-over mechanism is triggered.

- Periodically calls `optimism_syncStatus` to check the health status of OP Nodes.
- Periodically calls `eth_getBlockByNumber("latest")` to check the health status of OP Geths.
- Once the _leader Sequencer_ becomes unhealthy, the Fail-Over mechanism is activated.

## Fail-Over

1. To initiate fail-over, block production at the current _leader Sequencer_ is stopped by calling `admin_stopSequencer`. The method `admin_stopSequencer` returns a hash that indicates the _Sequencer_'s current unsafe block hash, referred to as _stopped hash_.

An edge case occurs when the target _Sequencer_ hasn't launched OP Geth properly; in this case, the returned _stopped hash_ is `0x00..00`. We treat `0x00..00` as an error, and call `optimism_syncStatus` to get the `unsafe_l2.hash` the _stopped hash_.

2. The system waits until any one healthy _Sequencer_'s unsafe block hash is equal to _stopped hash_.

It ensures that the new _Leader Sequencer_ grow to the same height as the old _Leader Sequencer_ before block production, to avoid the new Leader Sequencer producing blocks at any one old height, which could lead to a reorg.

3. To replace the _leader Sequencer_, any one healthy _Sequencer_ is elected by calling `admin_startSequencer(stopped hash)` if `stopped hash` is not `0x00..00`. Alternatively, the unsafe block hash from the new _leader Sequencer_ obtained via `optimism_syncStatus` is used for the election.

## Inspection (unimplemented yet)

This mechanism periodically inspects all the _Sequencers_. If a non-`leader` _Sequencer_ is found with its `admin_sequencerActive` flag set to `true`, it will be reset to `false`. The goal is to ensure the existence of only one _Active Sequencer_.
