package tests

// TODO: Restart、Stop OP Node 和 OP Geth 还不一样，要分别测试一下

import (
	"context"
	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	rollupNode "github.com/ethereum-optimism/optimism/op-node/node"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-node/testlog"
	"github.com/ethereum/go-ethereum/log"
	"github.com/node-real/op-coordinator/config"
	"github.com/node-real/op-coordinator/core"
	"github.com/node-real/op-coordinator/types"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func defaultCoordinator(candidates []*types.Node, log log.Logger) *core.Coordinator {
	candidatesCfg := make(map[string]*config.NodeConfig)
	for _, candidate := range candidates {
		candidatesCfg[candidate.Name] = &config.NodeConfig{} // dummy config
	}
	coordCfg := config.Config{
		Candidates: candidatesCfg,
		Bridges:    make(map[string]*config.NodeConfig),
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

	candidateNodes := make(map[string]*types.Node)
	for _, candidate := range candidates {
		candidateNodes[candidate.Name] = candidate
	}

	hc := core.NewHealthChecker(time.Duration(coordCfg.HealthCheck.IntervalMs)*time.Millisecond, coordCfg.HealthCheck.FailureThresholdLast5, log)
	go hc.Start(context.Background(), &candidateNodes)

	return core.NewCoordinator(coordCfg, hc, candidateNodes, log)
}

func extractNodes(sys *op_e2e.System, nodeNames []string) ([]*types.Node, error) {
	nodes := make([]*types.Node, 0)
	for _, nodeName := range nodeNames {
		n, err := types.NewNode(
			nodeName,
			sys.RollupNodes[nodeName].HTTPEndpoint(),
			sys.Nodes[nodeName].HTTPEndpoint(),
		)
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, n)
	}
	return nodes, nil
}

func TestSingleSequencer(t *testing.T) {
	log := testlog.Logger(t, log.LvlInfo)

	cfg := op_e2e.DefaultSystemConfig(t)
	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	candidates, err := extractNodes(sys, []string{"sequencer"})
	require.Nil(t, err, "Error getting nodes")

	coord := defaultCoordinator(candidates, log)

	// FIXME TODO
	time.Sleep(10 * time.Second)

	require.True(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.GetMaster())
	require.Nil(t, coord.MaybeElect())
	require.NotNil(t, coord.GetMaster())
	require.Equal(t, coord.GetMaster().Name, "sequencer")
	require.Nil(t, coord.GetStoppedHash())

	// Stop OP Node of sequencer
	require.Nil(t, sys.RollupNodes["sequencer"].Close())

	// FIXME TODO
	time.Sleep(10 * time.Second)

	require.False(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.RevokeMaster())
	require.NotNil(t, coord.GetStoppedHash())
	require.Nil(t, coord.GetMaster())
	require.NotNil(t, coord.MaybeElect())
	require.Nil(t, coord.GetMaster())
	require.NotNil(t, coord.GetStoppedHash())
}

func TestMultipleUnconnectedSequencers(t *testing.T) {
	log := testlog.Logger(t, log.LvlInfo)

	cfg := op_e2e.DefaultSystemConfig(t)

	// Attach follower
	cfg.Nodes["follower"] = &rollupNode.Config{
		Driver: driver.Config{
			VerifierConfDepth:  0,
			SequencerConfDepth: 0,
			SequencerEnabled:   true,
			SequencerStopped:   true,
		},
		// Submitter PrivKey is set in system start for rollup nodes where sequencer = true
		RPC: rollupNode.RPCConfig{
			ListenAddr:  "127.0.0.1",
			ListenPort:  0,
			EnableAdmin: true,
		},
		L1EpochPollInterval: time.Second * 4,
		ConfigPersistence:   &rollupNode.DisabledConfigPersistence{},
	}
	cfg.Loggers["follower"] = testlog.Logger(t, 3).New("role", "follower")

	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	candidates, err := extractNodes(sys, []string{"sequencer", "follower"})
	require.Nil(t, err, "Error getting nodes")

	coord := defaultCoordinator(candidates, log)

	// FIXME TODO
	time.Sleep(10 * time.Second)

	require.True(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.GetMaster())
	require.Nil(t, coord.MaybeElect())
	require.NotNil(t, coord.GetMaster())
	require.Equal(t, coord.GetMaster().Name, "sequencer")
	require.Nil(t, coord.GetStoppedHash())

	// Stop OP Node of sequencer
	require.Nil(t, sys.RollupNodes["sequencer"].Close())

	// FIXME TODO
	time.Sleep(10 * time.Second)

	require.False(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.RevokeMaster())
	require.NotNil(t, coord.GetStoppedHash())
	require.Nil(t, coord.GetMaster())

	err = coord.MaybeElect()
	require.NotNil(t, err)
	require.ErrorContains(t, err, "canonical does not contains prev stopped hash")
	require.Nil(t, coord.GetMaster())
}

func TestMultipleConnectedSequencers(t *testing.T) {
	log := testlog.Logger(t, log.LvlInfo)

	cfg := op_e2e.DefaultSystemConfig(t)

	// Attach follower
	cfg.Nodes["follower"] = &rollupNode.Config{
		Driver: driver.Config{
			VerifierConfDepth:  0,
			SequencerConfDepth: 0,
			SequencerEnabled:   true,
			SequencerStopped:   true,
		},
		// Submitter PrivKey is set in system start for rollup nodes where sequencer = true
		RPC: rollupNode.RPCConfig{
			ListenAddr:  "127.0.0.1",
			ListenPort:  0,
			EnableAdmin: true,
		},
		L1EpochPollInterval: time.Second * 4,
		ConfigPersistence:   &rollupNode.DisabledConfigPersistence{},
	}
	cfg.Loggers["follower"] = testlog.Logger(t, 3).New("role", "follower")

	// Enable P2P Connection
	cfg.P2PReqRespSync = true
	cfg.P2PTopology = make(map[string][]string)
	cfg.P2PTopology["sequencer"] = []string{"follower"}
	cfg.P2PTopology["follower"] = []string{"sequencer"}

	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	candidates, err := extractNodes(sys, []string{"sequencer", "follower"})
	require.Nil(t, err, "Error getting nodes")

	coord := defaultCoordinator(candidates, log)

	// FIXME TODO
	time.Sleep(10 * time.Second)

	require.True(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.GetMaster())
	require.Nil(t, coord.MaybeElect())
	require.NotNil(t, coord.GetMaster())
	require.Equal(t, coord.GetMaster().Name, "sequencer")
	require.Nil(t, coord.GetStoppedHash())

	// Stop OP Node of sequencer If nodes are already converged
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer ctxCancel()
	require.True(t, core.WaitForNodesConvergence(log, ctx, candidates))
	require.Nil(t, sys.RollupNodes["sequencer"].Close())

	// FIXME TODO
	time.Sleep(10 * time.Second)

	require.False(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.RevokeMaster())
	require.NotNil(t, coord.GetStoppedHash())
	require.Nil(t, coord.GetMaster())

	err = coord.MaybeElect()
	require.Nil(t, err)
	require.NotNil(t, coord.GetMaster())
	require.Equal(t, coord.GetMaster().Name, "follower")
}

func TestMultipleConnectedSequencers2(t *testing.T) {
	log := testlog.Logger(t, log.LvlInfo)

	cfg := op_e2e.DefaultSystemConfig(t)

	// Attach follower
	cfg.Nodes["follower"] = &rollupNode.Config{
		Driver: driver.Config{
			VerifierConfDepth:  0,
			SequencerConfDepth: 0,
			SequencerEnabled:   true,
			SequencerStopped:   true,
		},
		// Submitter PrivKey is set in system start for rollup nodes where sequencer = true
		RPC: rollupNode.RPCConfig{
			ListenAddr:  "127.0.0.1",
			ListenPort:  0,
			EnableAdmin: true,
		},
		L1EpochPollInterval: time.Second * 4,
		ConfigPersistence:   &rollupNode.DisabledConfigPersistence{},
	}
	cfg.Loggers["follower"] = testlog.Logger(t, 3).New("role", "follower")

	// Enable P2P Connection
	cfg.P2PReqRespSync = true
	cfg.P2PTopology = make(map[string][]string)
	cfg.P2PTopology["sequencer"] = []string{"follower"}
	cfg.P2PTopology["follower"] = []string{"sequencer"}

	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	candidates, err := extractNodes(sys, []string{"sequencer", "follower"})
	require.Nil(t, err, "Error getting nodes")

	coord := defaultCoordinator(candidates, log)

	// FIXME TODO
	time.Sleep(10 * time.Second)

	require.True(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.GetMaster())
	require.Nil(t, coord.MaybeElect())
	require.NotNil(t, coord.GetMaster())
	require.Equal(t, coord.GetMaster().Name, "sequencer")
	require.Nil(t, coord.GetStoppedHash())

	// Stop OP Node of sequencer If nodes are already converged
	ctx, ctxCancel := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	defer ctxCancel()
	require.True(t, core.WaitForNodesConvergence(log, ctx, candidates))
	sys.RollupNodes["sequencer"].Close()

	// FIXME TODO
	time.Sleep(10 * time.Second)

	require.False(t, coord.IsHealthy("sequencer"))
	require.Nil(t, coord.RevokeMaster())
	require.NotNil(t, coord.GetStoppedHash())
	require.Nil(t, coord.GetMaster())

	err = coord.MaybeElect()
	require.Nil(t, err)
	require.NotNil(t, coord.GetMaster())
	require.Equal(t, coord.GetMaster().Name, "follower")

	// FIXME TODO wait some blocks mined by follower
	time.Sleep(10 * time.Second)
	require.Nil(t, sys.RollupNodes["sequencer"].Start(context.Background()))

	//// Expect sequencer re-sync to follower after starting
	//ctx2, ctxCancel2 := context.WithTimeout(context.Background(), time.Duration(10)*time.Second)
	//defer ctxCancel2()
	//require.True(t, core.WaitForNodesConvergence(log, ctx2, candidates))
}

func TestBilibili(t *testing.T) {
	log := testlog.Logger(t, log.LvlDebug)

	cfg := op_e2e.DefaultSystemConfig(t)

	// Attach follower
	cfg.Nodes["follower"] = &rollupNode.Config{
		Driver: driver.Config{
			VerifierConfDepth:  0,
			SequencerConfDepth: 0,
			SequencerEnabled:   true,
			SequencerStopped:   true,
		},
		// Submitter PrivKey is set in system start for rollup nodes where sequencer = true
		RPC: rollupNode.RPCConfig{
			ListenAddr:  "127.0.0.1",
			ListenPort:  0,
			EnableAdmin: true,
		},
		L1EpochPollInterval: time.Second * 4,
		ConfigPersistence:   &rollupNode.DisabledConfigPersistence{},
	}
	cfg.Loggers["follower"] = testlog.Logger(t, 4).New("role", "follower")
	cfg.Nodes["follower2"] = &rollupNode.Config{
		Driver: driver.Config{
			VerifierConfDepth:  0,
			SequencerConfDepth: 0,
			SequencerEnabled:   true,
			SequencerStopped:   true,
		},
		// Submitter PrivKey is set in system start for rollup nodes where sequencer = true
		RPC: rollupNode.RPCConfig{
			ListenAddr:  "127.0.0.1",
			ListenPort:  0,
			EnableAdmin: true,
		},
		L1EpochPollInterval: time.Second * 4,
		ConfigPersistence:   &rollupNode.DisabledConfigPersistence{},
	}
	cfg.Loggers["follower2"] = testlog.Logger(t, 4).New("role", "follower2")

	cfg.Loggers["sequencer"] = testlog.Logger(t, 4).New("role", "sequencer")

	// Enable P2P Connection
	cfg.P2PReqRespSync = false
	cfg.P2PTopology = make(map[string][]string)
	cfg.P2PTopology["sequencer"] = []string{"follower"}
	cfg.P2PTopology["follower"] = []string{"sequencer", "follower2"}
	cfg.P2PTopology["follower2"] = []string{"follower"}

	sys, err := cfg.Start()
	require.Nil(t, err, "Error starting up system")
	defer sys.Close()

	// FIXME TODO
	time.Sleep(10 * time.Second)

	log.Info("bilibili", "sequencer.HTTP", sys.RollupNodes["sequencer"].HTTPEndpoint())
	log.Info("bilibili", "follower.HTTP", sys.RollupNodes["follower"].HTTPEndpoint())
	log.Info("bilibili", "follower2.HTTP", sys.RollupNodes["follower2"].HTTPEndpoint())

	sequencerName := "sequencer"
	sequencer, err := types.NewNode(
		sequencerName,
		sys.RollupNodes[sequencerName].HTTPEndpoint(),
		sys.Nodes[sequencerName].HTTPEndpoint(),
	)
	sequencerNum, _ := sequencer.OpGeth.BlockNumber(context.Background())
	log.Info("bilibili", "sequencerNum", sequencerNum)

	followerName := "follower"
	follower, err := types.NewNode(
		followerName,
		sys.RollupNodes[followerName].HTTPEndpoint(),
		sys.Nodes[followerName].HTTPEndpoint(),
	)
	followerNum, _ := follower.OpGeth.BlockNumber(context.Background())
	log.Info("bilibili", "followerNum", followerNum)

	follower2Name := "follower2"
	follower2, err := types.NewNode(
		follower2Name,
		sys.RollupNodes[follower2Name].HTTPEndpoint(),
		sys.Nodes[follower2Name].HTTPEndpoint(),
	)
	follower2Num, _ := follower2.OpGeth.BlockNumber(context.Background())
	log.Info("bilibili", "follower2Num", follower2Num)

	time.Sleep(100000 * time.Second)
}
