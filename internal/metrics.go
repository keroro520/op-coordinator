package internal

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	Metrics_errors = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "op",
		Subsystem: "coordinator",
		Name:      "error",
		Help:      "The internal error of Op_coordinator",
	}, []string{"type"})

	Metrics_check = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "op",
		Subsystem: "coordinator",
		Name:      "count",
		Help:      "The internal error of Op_coordinator",
	}, []string{"type"})
)

func startMetrics(config Config) {
	addr := fmt.Sprintf("%s:%d", config.Metrics.Host, config.Metrics.Port)
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(addr, nil); err != nil {
			panic(err)
		}
	}()
}
