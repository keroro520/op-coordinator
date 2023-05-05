package internal

import (
	"context"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"sync"
	"time"
)

// TODO configure
var HealthcheckWindow = 300
var HealthcheckInterval = time.Second
var HealthcheckThreshold = 10

var ErrUninitializedHealthcheck = fmt.Errorf("uninitialized healthcheck error")

func (c *Coordinator) IsHealthy(node *Node) bool {
	return c.healthcheckStat[*node] >= HealthcheckThreshold
}

func (c *Coordinator) HealthcheckInBackground(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(HealthcheckInterval)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.healthcheck()
			}
		}
	}()
}

func (c *Coordinator) healthcheck() {
	var wg sync.WaitGroup

	errors := make(map[*Node]error, len(c.candidates))
	for _, node := range c.candidates {
		wg.Add(1)
		go func(node *Node, client *NodeClient) {
			defer wg.Done()

			var err error
			if err = healthcheckOpGeth(context.Background(), client.opGeth); err == nil {
				err = healthcheckOpNode(context.Background(), client.opNode)
			}
			errors[node] = err
		}(&node, &node.client)
	}
	wg.Wait()

	c.updateHealthchecks(&errors)
}

func (c *Coordinator) updateHealthchecks(errors *map[*Node]error) {
	c.lastHealthcheck = (c.lastHealthcheck + 1) % HealthcheckWindow
	for node, err := range *errors {
		// Initialize healthchecks for this fresh node
		if c.healthchecks[*node] == nil {
			c.healthchecks[*node] = &map[int]error{}
			for i := 0; i < HealthcheckWindow; i++ {
				(*c.healthchecks[*node])[i] = ErrUninitializedHealthcheck
			}
			c.healthcheckStat[*node] = HealthcheckWindow
		}

		// Update healthchecks for the node
		previous := (*c.healthchecks[*node])[c.lastHealthcheck%HealthcheckWindow]
		(*c.healthchecks[*node])[c.lastHealthcheck%HealthcheckWindow] = err

		// Update healthcheckStat when the node's status changed
		if previous == nil && err != nil {
			c.healthcheckStat[*node]++
		} else if previous != nil && err == nil {
			c.healthcheckStat[*node]--
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
