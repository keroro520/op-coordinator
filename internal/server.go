package internal

import (
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	"github.com/node-real/op-coordinator/internal/bridge"
	"github.com/node-real/op-coordinator/internal/config"
	"github.com/node-real/op-coordinator/internal/coordinator"
	http_ "github.com/node-real/op-coordinator/internal/http"
	"github.com/node-real/op-coordinator/internal/metrics"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
)

type RpcServer struct {
	endpoint   string
	apis       []rpc.API
	httpServer *http.Server
	appVersion string
	listenAddr net.Addr
}

func NewRPCServer(cfg config.Config, appVersion string, c *coordinator.Coordinator, h *bridge.HighestBridge) *RpcServer {
	endpoint := net.JoinHostPort(cfg.RPC.Host, strconv.Itoa(cfg.RPC.Port))
	coordinatorAPI := http_.NewCoordinatorAPI(appVersion, c, h)
	rollupAPI := http_.NewRollupAPI(cfg, h)
	ethAPI := http_.NewEthAPI(h)
	r := &RpcServer{
		endpoint: endpoint,
		apis: []rpc.API{{
			Namespace:     "coordinator",
			Service:       coordinatorAPI,
			Public:        true,
			Authenticated: false,
		}, {
			Namespace:     "optimism",
			Service:       rollupAPI,
			Public:        true,
			Authenticated: false,
		}, {
			Namespace:     "eth",
			Service:       ethAPI,
			Public:        true,
			Authenticated: false,
		}},
		appVersion: appVersion,
	}
	return r
}

func NewHTTPRecordingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics.MetricHTTPRequests.WithLabelValues(r.Method).Inc()
		ww := httputil.NewWrappedResponseWriter(w)
		start := time.Now()
		next.ServeHTTP(ww, r)
		duration := time.Since(start)
		metrics.MetricHTTPResponses.WithLabelValues(r.Method, strconv.Itoa(ww.StatusCode)).Inc()
		metrics.MetricHTTPRequestDuration.WithLabelValues(r.Method, strconv.Itoa(ww.StatusCode)).Observe(float64(duration.Milliseconds()))
	})
}

func (s *RpcServer) Start() error {
	srv := rpc.NewServer()
	if err := node.RegisterApis(s.apis, nil, srv); err != nil {
		return err
	}
	nodeHandler := node.NewHTTPHandlerStack(srv, []string{"*"}, []string{"*"}, nil)
	nodeHandler = NewHTTPRecordingMiddleware(nodeHandler)

	mux := http.NewServeMux()
	mux.Handle("/", nodeHandler)
	mux.HandleFunc("/healthz", HealthzHandler(s.appVersion))

	s.httpServer = &http.Server{
		Handler:           mux,
		Addr:              s.endpoint,
		ReadTimeout:       rpc.DefaultHTTPTimeouts.ReadTimeout,
		ReadHeaderTimeout: rpc.DefaultHTTPTimeouts.ReadHeaderTimeout,
		WriteTimeout:      rpc.DefaultHTTPTimeouts.WriteTimeout,
		IdleTimeout:       rpc.DefaultHTTPTimeouts.IdleTimeout,
	}
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil {
			s.httpServer.ErrorLog.Printf("HTTP server exit, error: %v", err)
		}
	}()

	return nil
}

func HealthzHandler(appVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(appVersion))
	}
}
