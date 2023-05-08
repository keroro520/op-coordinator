package internal

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"go.uber.org/zap"
	"net"
	"net/http"
	"strconv"
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

type rpcServer struct {
	endpoint   string
	apis       []rpc.API
	httpServer *http.Server
	appVersion string
	listenAddr net.Addr
}

func NewRPCServer(rpcCfg RpcConfig, appVersion string, c *Coordinator) (*rpcServer, error) {
	api := NewNodeAPI(appVersion, c)
	endpoint := net.JoinHostPort(rpcCfg.Host, strconv.Itoa(rpcCfg.Port))
	r := &rpcServer{
		endpoint: endpoint,
		apis: []rpc.API{{
			Namespace:     "optimism",
			Service:       api,
			Public:        true,
			Authenticated: false,
		}},
		appVersion: appVersion,
	}
	return r, nil
}

func (s *rpcServer) Start() error {
	srv := rpc.NewServer()
	if err := node.RegisterApis(s.apis, nil, srv); err != nil {
		return err
	}
	nodeHandler := node.NewHTTPHandlerStack(srv, []string{"*"}, []string{"*"}, nil)
	mux := http.NewServeMux()
	mux.Handle("/", nodeHandler)
	mux.HandleFunc("/healthz", healthzHandler(s.appVersion))
	listener, err := net.Listen("tcp", s.endpoint)
	if err != nil {
		return err
	}
	s.listenAddr = listener.Addr()
	s.httpServer = NewHttpServer(mux)
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			//s.Error("http server failed", "err", err)
		}
	}()
	return nil
}
func healthzHandler(appVersion string) http.HandlerFunc {
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

func (n *nodeAPI) Master(nodeName string) (bool, error) {
	if nodeName == n.coordinator.master {
		return true, nil
	}
	go func() {
		if _, err := n.coordinator.nodes[nodeName].opNode.StopSequencer(context.Background()); err != nil {
			zap.S().Errorw("Fail to call admin_stopSequencer", "node", nodeName, "error", err)
		}
	}()
	return false, nil
}
