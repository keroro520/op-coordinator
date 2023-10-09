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

const HealthCheckInterval = 1000 * time.Millisecond
const HealthCheckFailureThresholdLast5 = 1

func defaultElection(t *testing.T, sysCfg op_e2e.SystemConfig, sys *op_e2e.System, log log.Logger) (*core.Election, *MockHealthChecker) {
	seqsCfg := make(map[string]*config.NodeConfig)
	seqNodes := make(map[string]*types.Node)
	bridgesCfg := make(map[string]*config.NodeConfig)
	bridgeNodes := make(map[string]*types.Node)
	for nodeName, opNode := range sys.RollupNodes {
		node, err := types.NewNode(nodeName, opNode.HTTPEndpoint(), sys.Nodes[nodeName].HTTPEndpoint())
		require.Nil(t, err)
		if sysCfg.Nodes[node.Name].Driver.SequencerEnabled {
			seqsCfg[node.Name] = &config.NodeConfig{} // dummy config
			seqNodes[node.Name] = node
		} else {
			bridgesCfg[node.Name] = &config.NodeConfig{} // dummy config
			bridgeNodes[node.Name] = node
		}
	}

	coordCfg := config.Config{
		Sequencers: seqsCfg,
		Bridges:    bridgesCfg,
		HealthCheck: config.HealthCheckConfig{
			IntervalMs:            HealthCheckInterval.Milliseconds(),
			FailureThresholdLast5: HealthCheckFailureThresholdLast5,
		},
		Election: config.ElectionConfig{
			Stopped:                        false,
			MaxWaitingTimeForConvergenceMs: 1000,
			MinRequiredHealthyNodes:        1,
		},
	}

	hc := MockHealthChecker{healthyNodes: make(map[string]bool)}
	coord := core.NewElection(coordCfg, &hc, seqNodes, log)

	return coord, &hc
}

type MockHealthChecker struct {
	healthyNodes map[string]bool
}

func (hc *MockHealthChecker) IsHealthy(nodeName string) bool {
	healthy, ok := hc.healthyNodes[nodeName]
	if !ok {
		return true
	} else {
		return healthy
	}
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

		if cfg.P2PTopology[seqName] == nil {
			cfg.P2PTopology[seqName] = make([]string, 0)
		}
		for _, peer := range p2pStaticPeers {
			cfg.P2PTopology[seqName] = append(cfg.P2PTopology[seqName], peer)

			if cfg.P2PTopology[peer] == nil {
				cfg.P2PTopology[peer] = make([]string, 0)
			}
			cfg.P2PTopology[peer] = append(cfg.P2PTopology[peer], seqName)
		}
	}
}

func TestSingleSequencerStopSequencerExpectEmptyLeader(t *testing.T) {
	log := testlog.Logger(t, log.LvlInfo)
	cfg := op_e2e.DefaultSystemConfig(t)
	cfg.DisableBatcher = true
	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	coord, hc := defaultElection(t, cfg, sys, log)
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return coord.IsHealthy("sequencer"), nil
	}))
	require.True(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.ActiveSequencer(), "Expect no active sequencer before election")

	err = coord.MaybeElect()
	require.Nil(t, err, "Expect election success")
	require.Equal(t, coord.ActiveSequencerName(), "sequencer", "Expect sequencer to be sequencer")
	require.Nil(t, coord.StoppedHash())

	becomeUnhealthy(hc, "sequencer")
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return !coord.IsHealthy("sequencer"), nil
	}))

	require.Nil(t, coord.RevokeActiveSequencer(), "Expect active sequencer revoked")
	require.Nil(t, coord.ActiveSequencer(), "Expect no active sequencer after revoke")
	require.NotNil(t, coord.StoppedHash())

	err = coord.MaybeElect()
	require.NotNil(t, err, "Expect election failed")
	require.Nil(t, coord.ActiveSequencer(), "Expect no active sequencer after election failed")
	require.NotNil(t, coord.StoppedHash())
}

func TestMultipleSequencersStopSequencerExpectEmptyLeaderIfPrevStoppedHashNotIncluded(t *testing.T) {
	log := testlog.Logger(t, log.LvlInfo)
	cfg := op_e2e.DefaultSystemConfig(t)
	cfg.DisableBatcher = true
	addSequencer(t, cfg, "sequencer2")

	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	coord, hc := defaultElection(t, cfg, sys, log)
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return coord.IsHealthy("sequencer") && coord.IsHealthy("sequencer2"), nil
	}))
	require.True(t, coord.IsHealthy("sequencer") && coord.IsHealthy("sequencer2"))
	require.Nil(t, coord.ActiveSequencer())

	err = coord.MaybeElect()
	require.Nil(t, err, "Expect election success")
	require.Equal(t, coord.ActiveSequencerName(), "sequencer")
	require.Nil(t, coord.StoppedHash())

	becomeUnhealthy(hc, "sequencer")
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return !coord.IsHealthy("sequencer") && coord.IsHealthy("sequencer2"), nil
	}))
	require.True(t, !coord.IsHealthy("sequencer") && coord.IsHealthy("sequencer2"))

	err = coord.RevokeActiveSequencer()
	require.Nil(t, err, "Expect active sequencer revoked as it is stopped")
	require.NotNil(t, coord.StoppedHash())
	require.Nil(t, coord.ActiveSequencer(), "Expect no active sequencer after revoke")

	err = coord.MaybeElect()
	require.ErrorContains(t, err, "canonical does not contains prev stopped hash")
	require.Nil(t, coord.ActiveSequencer())
}

func TestMultipleConnectedSequencers(t *testing.T) {
	log := testlog.Logger(t, log.LvlInfo)
	cfg := op_e2e.DefaultSystemConfig(t)
	cfg.DisableBatcher = true
	addSequencer(t, cfg, "sequencer2", "sequencer")

	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	coord, hc := defaultElection(t, cfg, sys, log)
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return coord.IsHealthy("sequencer"), nil
	}))
	require.True(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.ActiveSequencer(), "Expect no active sequencer before election")

	err = coord.MaybeElect()
	require.Nil(t, err, "Expect election success")
	require.Equal(t, coord.ActiveSequencerName(), "sequencer", "Expect sequencer to be sequencer")
	require.Nil(t, coord.StoppedHash())

	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer ctxCancel()
	require.True(t, core.WaitForNodesConvergence(log, ctx, coord.HealthySequencers()))

	becomeUnhealthy(hc, "sequencer")
	require.Nil(t, utils.WaitFor(context.Background(), 1*time.Second, func() (bool, error) {
		return !coord.IsHealthy("sequencer") && coord.IsHealthy("sequencer2"), nil
	}))
	require.True(t, !coord.IsHealthy("sequencer") && coord.IsHealthy("sequencer2"))

	err = coord.RevokeActiveSequencer()
	require.Nil(t, err, "Expect active sequencer revoked as it is stopped")
	require.NotNil(t, coord.StoppedHash())
	require.Nil(t, coord.ActiveSequencer())

	time.Sleep(3 * time.Second)

	err = coord.MaybeElect()
	require.Nil(t, err, "Expect election success")
	require.Equal(t, coord.ActiveSequencerName(), "sequencer2", "Expect sequencer2 to be active sequencer")
}

func becomeUnhealthy(hc *MockHealthChecker, nodeName string) {
	hc.healthyNodes[nodeName] = false
}

func becomeHealthy(hc *MockHealthChecker, nodeName string) {
	hc.healthyNodes[nodeName] = true
}
