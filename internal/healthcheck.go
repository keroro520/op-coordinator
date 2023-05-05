//package internal
//
//import (
//	"context"
//	"github.com/ethereum-optimism/optimism/op-node/eth"
//	"github.com/ethereum/go-ethereum/ethclient"
//	"github.com/ethereum/go-ethereum/rpc"
//	"sync"
//)
//
//type Node struct {
//	OpGethUrl string
//	OpNodeUrl string
//}
//
//type NodeClient struct {
//	opGeth *ethclient.Client
//	opNode *rpc.Client
//}
//
//// TODO opnode 通过什么方式传心跳呢？
//type Coordinator struct {
//	master     Node
//	masterLock sync.Mutex
//
//	candidates       []Node
//	candidatesClient map[Node]NodeClient
//
//	healthchecks       map[Node]*map[uint]error
//	healthcheckReports map[Node]uint
//	lastHealthcheck    uint
//}
//
//// TODO configure
//const HealthcheckWindow uint = 300
//const HealthcheckThreshold uint = 10
//
//func (c *Coordinator) IsHealthy(ndoe *Node) bool {
//	return c.healthcheckReports[*ndoe] >= HealthcheckThreshold
//}
//
//func (c *Coordinator) healthcheck(ctx context.Context) {
//	// TODO	sync.WaitGroup
//	c.lastHealthcheck++
//	for node, client := range c.candidatesClient {
//		var err error
//		if err = healthcheckOpGeth(ctx, client.opGeth); err == nil {
//			err = healthcheckOpNode(ctx, client.opNode)
//		}
//
//		if c.healthchecks[node] == nil {
//			// todo: 初始化为 "uninitialize"
//			c.healthchecks[node] = new(map[uint]error)
//			c.healthcheckReports[node] = 0
//		}
//
//		previous := (*c.healthchecks[node])[c.lastHealthcheck%HealthcheckWindow]
//		(*c.healthchecks[node])[c.lastHealthcheck%HealthcheckWindow] = err
//
//		if previous == nil && err != nil {
//			c.healthcheckReports[node]++
//		} else if previous != nil && err == nil {
//			c.healthcheckReports[node]--
//		}
//	}
//}
//
//func healthcheckOpGeth(ctx context.Context, client *ethclient.Client) error {
//	_, err := client.BlockByNumber(ctx, nil)
//	return err
//}
//
//func healthcheckOpNode(ctx context.Context, client *rpc.Client) error {
//	var syncStatus eth.SyncStatus
//	err := client.CallContext(ctx, &syncStatus, "optimism_syncStatus")
//	return err
//}
