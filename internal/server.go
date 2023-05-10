package internal

import (
	"context"
	"errors"
	"fmt"
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
			Namespace:     "coordinator",
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

func executeAdminCommand(ctx context.Context, coordinator *Coordinator, cmd AdminCommand) error {
	coordinator.AdminCh() <- cmd

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-cmd.RespCh():
		return err
	}
}
