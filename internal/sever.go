package internal

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
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

func NewRPCServer(ctx context.Context, rpcCfg RpcConfig, appVersion string) (*rpcServer, error) {
	api := NewNodeAPI(appVersion)
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
	version string
}

func NewNodeAPI(version string) *nodeAPI {
	return &nodeAPI{version: version}
}

func (nodeAPI *nodeAPI) Version(ctx context.Context) (string, error) {
	return nodeAPI.version, nil
}
