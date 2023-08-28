package rpc

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/node-real/op-coordinator/config"
	"github.com/node-real/op-coordinator/core"
	"net"
	"net/http"
	"strconv"
)

type RpcServer struct {
	endpoint   string
	apis       []rpc.API
	httpServer *http.Server
	appVersion string
	listenAddr net.Addr
}

func NewRPCServer(cfg config.Config, appVersion string, c *core.Election, log log.Logger) *RpcServer {
	endpoint := net.JoinHostPort(cfg.RPC.Host, strconv.Itoa(cfg.RPC.Port))
	electionAPI := NewElectionAPI(appVersion, c, log)
	rollupAPI := NewRollupAPI(cfg, c)
	ethAPI := NewEthAPI(c)
	r := &RpcServer{
		endpoint: endpoint,
		apis: []rpc.API{{
			Namespace:     "coordinator",
			Service:       electionAPI,
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

func (s *RpcServer) Start() error {
	srv := rpc.NewServer()
	if err := node.RegisterApis(s.apis, nil, srv); err != nil {
		return err
	}
	nodeHandler := node.NewHTTPHandlerStack(srv, []string{"*"}, []string{"*"}, nil)

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
