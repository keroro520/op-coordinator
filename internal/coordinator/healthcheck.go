package coordinator

import (
	"context"
	"github.com/node-real/op-coordinator/internal/metrics"
	"github.com/node-real/op-coordinator/internal/types"
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

func (c *HealthChecker) Start(ctx context.Context, nodes *map[string]*types.Node) {
	for nodeName, _ := range *nodes {
		c.windows[nodeName] = NewCumulativeSlidingWindow(c.cumulativeSlidingWindowSize)
	}

	// Start gorutines for each node to run health check every c.interval independently.
	var wg sync.WaitGroup
	for nodeName, node := range *nodes {
		wg.Add(1)
		go func(nodeName string, node *types.Node, slidingWindow *CumulativeSlidingWindow) {
			defer wg.Done()
			ticker := time.NewTicker(c.interval)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					var err error
					if err = healthcheckOpGeth(context.Background(), node); err == nil {
						err = healthcheckOpNode(context.Background(), node)
					}

					metrics.MetricHealthCheckTotal.WithLabelValues(node.Name).Inc()
					if err != nil {
						metrics.MetricHealthCheckFailures.WithLabelValues(node.Name).Inc()
						zap.S().Errorw("Health check error", "node", nodeName, "error", err)
					}

					isFailure := err != nil
					slidingWindow.Add(isFailure)
				}
			}
		}(nodeName, node, c.windows[nodeName])
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		const FALSE = float64(0)
		const TRUE = float64(1)
		freshMetricsTicker := time.NewTicker(time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			case <-freshMetricsTicker.C:
				for nodeName := range *nodes {
					value := TRUE
					if !c.IsHealthy(nodeName) {
						value = FALSE
					}
					metrics.MetricIsHealthy.WithLabelValues(nodeName).Set(value)
				}
			}
		}
	}()

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

func healthcheckOpGeth(ctx context.Context, node *types.Node) error {
	start := time.Now()
	_, err := node.OpGeth.HeaderByNumber(ctx, nil)
	duration := time.Since(start)
	metrics.MetricHealthCheckOpGethDuration.WithLabelValues(node.Name).Observe(float64(duration.Milliseconds()))
	return err
}

func healthcheckOpNode(ctx context.Context, node *types.Node) error {
	start := time.Now()
	_, err := node.OpNode.SyncStatus(ctx)
	duration := time.Since(start)
	metrics.MetricHealthCheckOpNodeDuration.WithLabelValues(node.Name).Observe(float64(duration.Milliseconds()))
	return err
}
