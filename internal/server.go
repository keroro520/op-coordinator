package internal

import (
	"context"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-service/httputil"
	"github.com/node-real/op-coordinator/internal/metrics"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"go.uber.org/zap"
)

var timeouts = rpc.DefaultHTTPTimeouts

func NewHttpServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadTimeout:       timeouts.ReadTimeout,
		ReadHeaderTimeout: timeouts.ReadHeaderTimeout,
		WriteTimeout:      timeouts.WriteTimeout,
		IdleTimeout:       timeouts.IdleTimeout,
	}
}

type RpcServer struct {
	endpoint   string
	apis       []rpc.API
	httpServer *http.Server
	appVersion string
	listenAddr net.Addr
}

func NewRPCServer(rpcCfg RpcConfig, appVersion string, c *Coordinator) *RpcServer {
	api := NewNodeAPI(appVersion, c)
	endpoint := net.JoinHostPort(rpcCfg.Host, strconv.Itoa(rpcCfg.Port))
	r := &RpcServer{
		endpoint: endpoint,
		apis: []rpc.API{{
			Namespace:     "coordinator",
			Service:       api,
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
		ReadTimeout:       timeouts.ReadTimeout,
		ReadHeaderTimeout: timeouts.ReadHeaderTimeout,
		WriteTimeout:      timeouts.WriteTimeout,
		IdleTimeout:       timeouts.IdleTimeout,
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

type nodeAPI struct {
	version     string
	coordinator *Coordinator
}

func NewNodeAPI(version string, c *Coordinator) *nodeAPI {
	return &nodeAPI{version: version, coordinator: c}
}

func (n *nodeAPI) Version() (string, error) {
	return n.version, nil
}

func (n *nodeAPI) RequestBuildingBlock(nodeName string) error {
	if n.coordinator.master == "" {
		return fmt.Errorf("empty master")
	}

	if n.coordinator.master != nodeName {
		go func() {
			zap.S().Warnw("Invalid master is requesting!", "master", n.coordinator.master, "node", nodeName)

			node := n.coordinator.nodes[nodeName]
			if node != nil && node.opNode != nil {
				blockHash, err := node.opNode.StopSequencer(context.Background())
				if err != nil {
					zap.S().Errorw("Fail to call admin_stopSequencer", "node", nodeName, "error", err)
				} else {
					zap.S().Infow("Success to call admin_stopSequencer", "blockHash", blockHash.String())
				}
			}
		}()
		return fmt.Errorf("unknown master")
	}

	return nil
}

func (n *nodeAPI) GetMaster() string {
	return n.coordinator.master
}

func (n *nodeAPI) SetMaster(nodeName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return executeAdminCommand(ctx, n.coordinator, NewSetMasterCommand(nodeName))
}

// executeAdminCommand executes an admin command and returns the result.
func executeAdminCommand(ctx context.Context, coordinator *Coordinator, cmd AdminCommand) error {
	coordinator.AdminCh() <- cmd

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-cmd.RespCh():
		return err
	}
}
