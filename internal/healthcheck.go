package internal

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"go.uber.org/zap"
	"sync"
	"time"
)

const CumulativeSlidingWindowSize = 5

// HealthChecker is used to check the health status of nodes and record the check results in a sliding window.
//
// The size of the sliding window is 5, that is, the last 5 check results are recorded. And we judge the health status
// of the node is obtained by the number of failures accumulated in the sliding window. If the number of failures
// exceeds the configuration `failureThresholdLast5`, the node is considered unhealthy.
type HealthChecker struct {
	windows                     map[string]*CumulativeSlidingWindow
	interval                    time.Duration
	cumulativeSlidingWindowSize int
	failureThresholdLast5       int
}

func NewHealthChecker(interval time.Duration, failureThresholdLast5 int) *HealthChecker {
	if failureThresholdLast5 >= CumulativeSlidingWindowSize {
		panic("failureThresholdLast5 should be less than CumulativeSlidingWindowSize")
	}
	return &HealthChecker{
		windows:                     make(map[string]*CumulativeSlidingWindow),
		interval:                    interval,
		cumulativeSlidingWindowSize: CumulativeSlidingWindowSize,
		failureThresholdLast5:       failureThresholdLast5,
	}
}

// IsHealthy returns true if the node is healthy, its recent health check failures are equal to or less than the
// threshold.
func (c *HealthChecker) IsHealthy(nodeName string) bool {
	return c.windows[nodeName] != nil && c.windows[nodeName].Failures() <= c.failureThresholdLast5
}

func (c *HealthChecker) Start(ctx context.Context, nodes *map[string]*Node) {
	for nodeName, _ := range *nodes {
		c.windows[nodeName] = NewCumulativeSlidingWindow(c.cumulativeSlidingWindowSize)
	}

	// Start gorutines for each node to run health check every c.interval independently.
	var wg sync.WaitGroup
	for nodeName, node := range *nodes {
		wg.Add(1)
		go func(nodeName string, node *Node, slidingWindow *CumulativeSlidingWindow) {
			defer wg.Done()
			ticker := time.NewTicker(c.interval)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					var err error
					if err = healthcheckOpGeth(context.Background(), node.opGeth); err == nil {
						err = healthcheckOpNode(context.Background(), node.opNode)
					}
					if err != nil {
						zap.S().Errorw("Health check error", "node", nodeName, "error", err)
					}
					isFailure := err != nil
					slidingWindow.Add(isFailure)
				}
			}
		}(nodeName, node, c.windows[nodeName])
	}

	wg.Wait()
}

// CumulativeSlidingWindow is a sliding window that records the last `size` check results.
type CumulativeSlidingWindow struct {
	// The sliding window.
	window []bool
	// The size of the sliding window.
	size int
	// The index of the sliding window.
	cursor int
	// The number of failures in the sliding window.
	failures int
}

func NewCumulativeSlidingWindow(size int) *CumulativeSlidingWindow {
	// `failures` is initialized to size, because we have not checked the health status of the node yet. We don't
	// know if the node is healthy or not, so we assume that the node is unhealthy.
	// As long as the node is healthy, the number of failures will be reduced later.
	failures := size
	window := make([]bool, size)
	for i := 0; i < size; i++ {
		window[i] = true
	}

	return &CumulativeSlidingWindow{
		window:   window,
		size:     size,
		cursor:   0,
		failures: failures,
	}
}

func (w *CumulativeSlidingWindow) Failures() int {
	return w.failures
}

func (w *CumulativeSlidingWindow) Add(failure bool) {
	// Subtract since that is being overwritten by the new one.
	if w.window[w.cursor] {
		w.failures--
	}

	// Set the new value and update the failure.
	if failure {
		w.failures++
	}
	w.window[w.cursor] = failure

	w.cursor = (w.cursor + 1) % w.size
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
