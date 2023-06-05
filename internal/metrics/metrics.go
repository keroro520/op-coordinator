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
	MetricElections = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "elections_count_total",
		Help:      "Tracks total elections",
	})
	MetricElectionFailures = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "election_failures_count_total",
		Help:      "Tracks total election failures",
	})
	MetricElectionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "election_duration_ms",
		Buckets:   prometheus.DefBuckets,
		Help:      "Tracks election duration in milliseconds",
	})
	MetricWaitingConvergenceDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "waiting_convergence_duration_ms",
		Buckets:   prometheus.DefBuckets,
		Help:      "Tracks waiting convergence duration in milliseconds",
	})
	MetricIsMaster = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "is_master",
		Help:      "Tracks whether the node is master",
	}, []string{"node"})
	MetricHealthCheckFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "hc_failures_count",
		Help:      "Tracks health check failures",
	}, []string{"node"})
	MetricHealthCheckOpNodeDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "hc_op_node_duration_ms",
		Buckets:   prometheus.DefBuckets,
	}, []string{"node"})
	MetricHealthCheckOpGethDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "hc_op_geth_duration_ms",
		Buckets:   prometheus.DefBuckets,
	}, []string{"node"})

	MetricHTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "http_request_duration_ms",
		Help:      "Tracks HTTP request durations, in ms",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "status_code"})
	MetricHTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "http_requests_count_total",
		Help:      "Tracks total HTTP requests",
	}, []string{"method"})
	MetricHTTPResponses = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Subsystem: Subsystem,
		Name:      "http_responses_count_total",
		Help:      "Tracks total HTTP responses",
	}, []string{"method", "status_code"})
)

func StartMetrics(addr string) {
	go func() {
		http.Handle("/", promhttp.Handler())
		if err := http.ListenAndServe(addr, nil); err != nil {
			panic(err)
		}
	}()
}
