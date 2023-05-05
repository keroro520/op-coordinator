package internal

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/node-real/op-coordinator/internal/client"
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
	opNode *client.RollupClient
	opGeth *ethclient.Client
}

// TODO opnode 通过什么方式传心跳呢？
type Coordinator struct {
	config     Config
	master     Node
	masterLock sync.Mutex

	candidates []Node

	healthchecks       map[Node]*map[uint]error
	healthcheckReports map[Node]uint
	lastHealthcheck    uint
	wg                 sync.WaitGroup
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
		rollupClient, err := dialRollupClientWithTimeout(ctx, v.nodeConfig.OpNodePublicRpcUrl)
		if err != nil {
			zap.S().Info("dial op node failed %s", v.nodeConfig.OpNodePublicRpcUrl)
			continue
		}
		gethClient, err := dialEthClientWithTimeout(ctx, v.nodeConfig.OpGethPublicRpcUrl)
		if err != nil {
			zap.S().Info("dial op geth failed %s", v.nodeConfig.OpGethPublicRpcUrl)
			continue
		}
		v.client = NodeClient{opNode: rollupClient, opGeth: gethClient}
	}
}

func (c *Coordinator) selectMaster(ctx context.Context) {
	for _, v := range c.candidates {
		master, err := v.client.opNode.IsMaster(ctx)
		if err != nil {
			continue
		}
		if master {
			c.master = v
			return
		}
	}
}

func (c *Coordinator) IsHealthy(ndoe *Node) bool {
	return c.healthcheckReports[*ndoe] >= c.config.HealthCheckThreshold
}

func (c *Coordinator) healthcheck(ctx context.Context) {
	// TODO	sync.WaitGroup
	c.lastHealthcheck++
	for node, client := range c.candidatesClient {
		var err error
		if err = healthcheckOpGeth(ctx, client.opGeth); err == nil {
			err = healthcheckOpNode(ctx, client.opNode)
		}

		if c.healthchecks[node] == nil {
			// todo: 初始化为 "uninitialize"
			c.healthchecks[node] = new(map[uint]error)
			c.healthcheckReports[node] = 0
		}

		previous := (*c.healthchecks[node])[c.lastHealthcheck%c.config.HealthCheckWindow]
		(*c.healthchecks[node])[c.lastHealthcheck%c.config.HealthCheckWindow] = err

		if previous == nil && err != nil {
			c.healthcheckReports[node]++
		} else if previous != nil && err == nil {
			c.healthcheckReports[node]--
		}
	}
}

func healthcheckOpGeth(ctx context.Context, client *ethclient.Client) error {
	_, err := client.BlockByNumber(ctx, nil)
	return err
}

func healthcheckOpNode(ctx context.Context, client *rpc.Client) error {
	var syncStatus eth.SyncStatus
	err := client.CallContext(ctx, &syncStatus, "optimism_syncStatus")
	return err
}
