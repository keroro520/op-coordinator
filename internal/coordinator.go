package internal

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"go.uber.org/zap"
	"sort"
	"sync"
	"time"
)

type Node struct {
	name       string
	nodeConfig NodeConfig
	client     NodeClient
}

type NodeClient struct {
	opNode *rpc.Client
	opGeth *ethclient.Client
}

type Coordinator struct {
	config     Config
	master     *Node
	masterLock sync.Mutex

	candidates []Node

	healthchecks    map[Node]*map[int]error
	healthcheckStat map[Node]int
	lastHealthcheck int
}

func NewCoordinator(ctx context.Context, config Config) *Coordinator {
	coordinator := &Coordinator{
		config: config,
	}
	coordinator.candidates = newCandidates(config)
	coordinator.connectNode(ctx)
	return coordinator
}

func newCandidates(config Config) []Node {
	candidates := make([]Node, len(config.Candidates))
	for k, v := range config.Candidates {
		candidates = append(candidates, Node{
			name:       k,
			nodeConfig: *v,
		})
	}
	return candidates
}

func Start(config Config, ctx context.Context) {
	startMetrics(config)
	s, e := NewRPCServer(ctx, config.RPC, "v1.0")
	if e != nil {
		panic(e)
	}
	s.Start()
	coordinator := NewCoordinator(ctx, config)
	coordinator.loop(ctx)
}

func (c *Coordinator) loop(ctx context.Context) {
	zap.S().Info("Coordinator start loop")
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ctx.Done():
			zap.S().Info("Coordinator exit loop")
			return
		case <-ticker.C:
			if c.master == nil {
				c.selectMaster(ctx)
				continue
			}

			if !c.IsHealthy(c.master) {
				c.revokeCurrentMaster()
			}
		}
	}
}

func (c *Coordinator) connectNode(ctx context.Context) {
	for _, candidate := range c.candidates {
		opNodeClient, err := rpc.Dial(candidate.nodeConfig.OpNodePublicRpcUrl)
		if err != nil {
			zap.S().Error("dial op node failed %s", candidate.nodeConfig.OpNodePublicRpcUrl)
			continue
		}
		gethClient, err := dialEthClientWithTimeout(ctx, candidate.nodeConfig.OpGethPublicRpcUrl)
		if err != nil {
			zap.S().Error("dial op geth failed %s", candidate.nodeConfig.OpGethPublicRpcUrl)
			continue
		}
		candidate.client = NodeClient{opNode: opNodeClient, opGeth: gethClient}
	}
}

func (c *Coordinator) selectMaster(ctx context.Context) {
	for _, candidate := range c.candidates {
		var sequencerStopped bool
		err := candidate.client.opNode.CallContext(ctx, &sequencerStopped, "admin_sequencerStopped")
		if err != nil {
			continue
		}

		if sequencerStopped == false {
			c.master = &candidate
			return
		}
	}

	nodeStates := make(map[*eth.SyncStatus]*Node)
	for _, candidate := range c.candidates {
		var syncStatus *eth.SyncStatus
		err := candidate.client.opNode.CallContext(ctx, syncStatus, "sync_status")
		if err != nil {
			continue
		}
		nodeStates[syncStatus] = &candidate
	}
	var nodeStatesSlice []*eth.SyncStatus
	for nodeState := range nodeStates {
		nodeStatesSlice = append(nodeStatesSlice, nodeState)
	}
	sort.Slice(nodeStatesSlice, func(i, j int) bool {
		return nodeStatesSlice[i].UnsafeL2.Number > nodeStatesSlice[j].UnsafeL2.Number
	})
	err := nodeStates[nodeStatesSlice[0]].client.opNode.CallContext(ctx, nil, "admin_startSequencer", nodeStatesSlice[0].UnsafeL2.Hash)
	if err != nil {
		zap.S().Error("start sequencer failed %s", nodeStates[nodeStatesSlice[0]].nodeConfig.OpNodePublicRpcUrl)
		return
	}
	c.master = nodeStates[nodeStatesSlice[0]]
}

func (c *Coordinator) revokeCurrentMaster() {
	zap.S().Warn("Revoke unhealthy master %s %s", c.master.name, c.master.nodeConfig.OpNodePublicRpcUrl)

	var hash common.Hash
	err := c.master.client.opNode.CallContext(context.Background(), &hash, "admin_stopSequencer")
	if err != nil {
		zap.S().Error("Fail to call admin_stopSequencer on %s even though its leadership was revoked, error: %+v", c.master.name, err)
		c.master = nil
	} else {
		c.master = nil
	}
}
