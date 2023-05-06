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

var ErrUninitializedHealthcheck = fmt.Errorf("uninitialized healthcheck")

func (c *Coordinator) IsHealthy(nodeName string) bool {
	return c.healthcheckStat[nodeName] >= c.config.HealthCheckThreshold
}

func (c *Coordinator) HealthcheckInBackground(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Duration(c.config.HealthCheckIntervalMs) * time.Millisecond)
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
	window := c.config.HealthCheckWindow
	c.lastHealthcheck = (c.lastHealthcheck + 1) % window

	var wg sync.WaitGroup
	for nodeName, node := range c.nodes {
		wg.Add(1)
		go func(nodeName string, node *Node) {
			defer wg.Done()

			var err error
			if err = healthcheckOpGeth(context.Background(), node.opGeth); err == nil {
				err = healthcheckOpNode(context.Background(), node.opNode)
			}
			c.onHealthcheckResult(nodeName, err)
		}(nodeName, node)
	}
	wg.Wait()
}

func (c *Coordinator) onHealthcheckResult(nodeName string, result error) {
	window := c.config.HealthCheckWindow
	previousResult := (*c.healthchecks[nodeName])[c.lastHealthcheck%window]

	// Initialize for fresh nodes
	if c.healthchecks[nodeName] == nil {
		c.healthchecks[nodeName] = &map[int]error{}
		for i := 0; i < window; i++ {
			(*c.healthchecks[nodeName])[i] = ErrUninitializedHealthcheck
		}
		c.healthcheckStat[nodeName] = window
	}

	// Update c.healthchecks for the node
	(*c.healthchecks[nodeName])[c.lastHealthcheck%window] = result

	// Update healthcheckStat when the node's status changed
	if previousResult == nil && result != nil {
		c.healthcheckStat[nodeName]++
	} else if previousResult != nil && result == nil {
		c.healthcheckStat[nodeName]--
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
