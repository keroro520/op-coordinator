package internal

import (
	"context"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"go.uber.org/zap"
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

// TODO opnode 通过什么方式传心跳呢？
type Coordinator struct {
	config     Config
	master     Node
	masterLock sync.Mutex

	candidates []Node

	healthchecks    map[Node]*map[int]error
	healthcheckStat map[Node]int
	lastHealthcheck int

	wg sync.WaitGroup
}

func NewCoordinator(config Config) *Coordinator {
	coordinator := &Coordinator{
		config: config,
		master: Node{name: ""},
	}
	coordinator.candidates = newCandidates(config)
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
	coordinator := NewCoordinator(config)
	coordinator.loop(ctx)
}

func (c *Coordinator) loop(ctx context.Context) {
	for {
		zap.S().Info("loop start.......")
		time.Sleep(time.Duration(c.config.SleepTime) * time.Second)
		c.wg.Add(1)
		go c.connectNode(ctx)
		if c.master.name == "" {
			c.selectMaster(ctx)
			return
		}
	}
}

func (c *Coordinator) connectNode(ctx context.Context) {
	for _, v := range c.candidates {
		opNodeClient, err := rpc.Dial(v.nodeConfig.OpNodePublicRpcUrl)
		if err != nil {
			zap.S().Info("dial op node failed %s", v.nodeConfig.OpNodePublicRpcUrl)
			continue
		}
		gethClient, err := dialEthClientWithTimeout(ctx, v.nodeConfig.OpGethPublicRpcUrl)
		if err != nil {
			zap.S().Info("dial op geth failed %s", v.nodeConfig.OpGethPublicRpcUrl)
			continue
		}
		v.client = NodeClient{opNode: opNodeClient, opGeth: gethClient}
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
			c.master = candidate
			break
		}
	}
}
