package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	Namespace = ""
	Subsystem = "coordinator"
)

var (
	MetricElectionEnabled = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "elections_enabled",
		Help:      "Indicates whether elections are enabled",
	})

	MetricIsActiveSequencer = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "elections_is_master",
		Help:      "Tracks total elections",
	}, []string{"node"})

	MetricIsHealthy = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "is_healthy",
		Help:      "Tracks whether the node is healthy",
	}, []string{"node"})

	MetricHealthCheckOpNodeSuccessDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "hc_op_node_success_duration_ms",
		Buckets:   prometheus.DefBuckets,
	}, []string{"node"})
	MetricHealthCheckOpNodeFailureDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "hc_op_node_failure_duration_ms",
		Buckets:   prometheus.DefBuckets,
	}, []string{"node"})
	MetricHealthCheckOpGethSuccessDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "hc_op_geth_success_duration_ms",
		Buckets:   prometheus.DefBuckets,
	}, []string{"node"})
	MetricHealthCheckOpGethFailureDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "hc_op_geth_failure_duration_ms",
		Buckets:   prometheus.DefBuckets,
	}, []string{"node"})
)

func StartMetrics(addr string) {
	go func() {
		http.Handle("/", promhttp.Handler())
		if err := http.ListenAndServe(addr, nil); err != nil {
			panic(err)
		}
	}()
}
