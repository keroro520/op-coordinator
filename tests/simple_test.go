package tests

import (
	"context"
	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	rollupNode "github.com/ethereum-optimism/optimism/op-node/node"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/testlog"
	"github.com/ethereum-optimism/optimism/op-service/client/utils"
	"github.com/ethereum/go-ethereum/log"
	"github.com/node-real/op-coordinator/config"
	"github.com/node-real/op-coordinator/core"
	"github.com/node-real/op-coordinator/types"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func defaultElection(t *testing.T, sysCfg op_e2e.SystemConfig, sys *op_e2e.System, log log.Logger) *core.Election {
	candidatesCfg := make(map[string]*config.NodeConfig)
	candidateNodes := make(map[string]*types.Node)
	bridgesCfg := make(map[string]*config.NodeConfig)
	bridgeNodes := make(map[string]*types.Node)
	for nodeName, opNode := range sys.RollupNodes {
		node, err := types.NewNode(nodeName, opNode.HTTPEndpoint(), sys.Nodes[nodeName].HTTPEndpoint())
		require.Nil(t, err)
		if sysCfg.Nodes[node.Name].Driver.SequencerEnabled {
			candidatesCfg[node.Name] = &config.NodeConfig{} // dummy config
			candidateNodes[node.Name] = node
		} else {
			bridgesCfg[node.Name] = &config.NodeConfig{} // dummy config
			bridgeNodes[node.Name] = node
		}
	}

	coordCfg := config.Config{
		Candidates: candidatesCfg,
		Bridges:    bridgesCfg,
		HealthCheck: config.HealthCheckConfig{
			IntervalMs:            1000,
			FailureThresholdLast5: 1,
		},
		Election: config.ElectionConfig{
			Stopped:                        false,
			MaxWaitingTimeForConvergenceMs: 1000,
			MinRequiredHealthyNodes:        1,
		},
	}

	hc := core.NewHealthChecker(candidateNodes, time.Duration(coordCfg.HealthCheck.IntervalMs)*time.Millisecond, coordCfg.HealthCheck.FailureThresholdLast5, log)
	coord := core.NewElection(coordCfg, hc, candidateNodes, log)
	go hc.Start(context.Background())

	return coord
}

func addSequencer(t *testing.T, cfg op_e2e.SystemConfig, seqName string, p2pStaticPeers ...string) {
	require.NotContains(t, cfg.Nodes, seqName)

	cfg.Nodes[seqName] = &rollupNode.Config{
		Driver: driver.Config{
			VerifierConfDepth:  0,
			SequencerConfDepth: 0,
			SequencerEnabled:   true,
			SequencerStopped:   true,
		},
		RPC: rollupNode.RPCConfig{
			ListenAddr:  "127.0.0.1",
			ListenPort:  0,
			EnableAdmin: true,
		},
		L1EpochPollInterval: time.Second * 1,
		ConfigPersistence:   &rollupNode.DisabledConfigPersistence{},
	}
	cfg.Loggers[seqName] = testlog.Logger(t, 3).New("role", seqName)

	if len(p2pStaticPeers) > 0 {
		cfg.P2PReqRespSync = true
		if cfg.P2PTopology == nil {
			cfg.P2PTopology = make(map[string][]string)
		}

		cfg.P2PTopology[seqName] = make([]string, 0)
		for _, peer := range p2pStaticPeers {
			cfg.P2PTopology[seqName] = append(cfg.P2PTopology[seqName], peer)
		}
	}
}

func TestSingleSequencerStopSequencerExpectEmptyLeader(t *testing.T) {
	log := testlog.Logger(t, log.LvlInfo)
	cfg := op_e2e.DefaultSystemConfig(t)
	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	coord := defaultElection(t, cfg, sys, log)
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return coord.IsHealthy("sequencer"), nil
	}))
	require.True(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.Master())
	require.Nil(t, coord.MaybeElect())
	require.NotNil(t, coord.Master())
	require.Equal(t, coord.Master().Name, "sequencer")
	require.Nil(t, coord.StoppedHash())

	// Stop OP Node of sequencer, expect empty leader even if re-elect because
	require.Nil(t, sys.RollupNodes["sequencer"].Close())
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return !coord.IsHealthy("sequencer"), nil
	}))
	require.Nil(t, coord.RevokeMaster())
	require.NotNil(t, coord.StoppedHash())
	require.Nil(t, coord.Master())
	require.NotNil(t, coord.MaybeElect())
	require.Nil(t, coord.Master())
	require.NotNil(t, coord.StoppedHash())
}

func TestMultipleSequencersStopSequencerExpectEmptyLeaderIfPrevStoppedHashNotIncluded(t *testing.T) {
	log := testlog.Logger(t, log.LvlInfo)
	cfg := op_e2e.DefaultSystemConfig(t)
	addSequencer(t, cfg, "sequencer2")

	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	coord := defaultElection(t, cfg, sys, log)
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return coord.IsHealthy("sequencer"), nil
	}))
	require.True(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.Master())
	require.Nil(t, coord.MaybeElect())
	require.NotNil(t, coord.Master())
	require.Equal(t, coord.Master().Name, "sequencer")
	require.Nil(t, coord.StoppedHash())

	// Stop OP Node of sequencer, expect empty leader even if re-elect, because candidates prev stopped hash not included
	require.Nil(t, sys.RollupNodes["sequencer"].Close())
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return !coord.IsHealthy("sequencer"), nil
	}))
	require.False(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.RevokeMaster())
	require.NotNil(t, coord.StoppedHash())
	require.Nil(t, coord.Master())
	require.ErrorContains(t, coord.MaybeElect(), "canonical does not contains prev stopped hash")
	require.Nil(t, coord.Master())
}

func TestMultipleConnectedSequencers(t *testing.T) {
	log := testlog.Logger(t, log.LvlInfo)
	cfg := op_e2e.DefaultSystemConfig(t)
	addSequencer(t, cfg, "sequencer2", "sequencer")

	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	coord := defaultElection(t, cfg, sys, log)
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return coord.IsHealthy("sequencer"), nil
	}))
	require.True(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.Master())
	require.Nil(t, coord.MaybeElect())
	require.NotNil(t, coord.Master())
	require.Equal(t, coord.Master().Name, "sequencer")
	require.Nil(t, coord.StoppedHash())

	// Stop OP Node of sequencer If nodes are already converged
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer ctxCancel()
	require.True(t, core.WaitForNodesConvergence(log, ctx, coord.HealthyCandidates()))
	require.Nil(t, sys.RollupNodes["sequencer"].Close())
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return !coord.IsHealthy("sequencer"), nil
	}))
	require.False(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.RevokeMaster())
	require.NotNil(t, coord.StoppedHash())
	require.Nil(t, coord.Master())

	require.Nil(t, coord.MaybeElect())
	require.NotNil(t, coord.Master())
	require.Equal(t, coord.Master().Name, "sequencer2")
}

func TestMultipleConnectedSequencers2(t *testing.T) {
	log := testlog.Logger(t, log.LvlInfo)
	cfg := op_e2e.DefaultSystemConfig(t)
	addSequencer(t, cfg, "sequencer2", "sequencer")

	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	coord := defaultElection(t, cfg, sys, log)
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return coord.IsHealthy("sequencer"), nil
	}))
	require.True(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.Master())
	require.Nil(t, coord.MaybeElect())
	require.NotNil(t, coord.Master())
	require.Equal(t, coord.Master().Name, "sequencer")
	require.Nil(t, coord.StoppedHash())

	// Stop OP Node of sequencer If nodes are already converged
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer ctxCancel()
	require.True(t, core.WaitForNodesConvergence(log, ctx, coord.HealthyNodes()))
	require.Nil(t, sys.RollupNodes["sequencer"].Close())
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return !coord.IsHealthy("sequencer"), nil
	}))
	require.False(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.RevokeMaster())
	require.NotNil(t, coord.StoppedHash())
	require.Nil(t, coord.Master())

	require.Nil(t, coord.MaybeElect())
	require.NotNil(t, coord.Master())
	require.Equal(t, coord.Master().Name, "sequencer2")

	// wait some blocks mined by follower
	time.Sleep(10 * time.Second)

	// FIXME: Currently OP Node is not supported to restart yet
	// Restart sequencer
	// require.Nil(t, sys.RollupNodes["sequencer"].Start(context.Background()))

	//// Expect sequencer re-sync to follower after starting
	//ctx2, ctxCancel2 := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	//defer ctxCancel2()
	//require.True(t, core.WaitForNodesConvergence(log, ctx2, candidates))
}
