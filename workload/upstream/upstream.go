package upstream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/andydunstall/piko/agent"
	agentconfig "github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/log"
	"go.uber.org/zap"
)

type server struct {
	ln     net.Listener
	server *http.Server
}

func newServer() (*server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//nolint
		io.Copy(w, r.Body)
	})
	return &server{
		server: &http.Server{
			Addr:    ln.Addr().String(),
			Handler: mux,
		},
		ln: ln,
	}, nil
}

func (s *server) Addr() string {
	return s.ln.Addr().String()
}

func (s *server) Serve() error {
	return s.server.Serve(s.ln)
}

func (s *server) Close() error {
	return s.server.Close()
}

type Upstream struct {
	endpointID string
	serverURL  string
	logger     log.Logger
}

func NewUpstream(endpointID string, serverURL string, logger log.Logger) *Upstream {
	return &Upstream{
		endpointID: endpointID,
		serverURL:  serverURL,
		logger:     logger,
	}
}

func (u *Upstream) Run(ctx context.Context) error {
	server, err := newServer()
	if err != nil {
		return err
	}
	defer server.Close()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.Serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			u.logger.Error("failed to serve upstream", zap.Error(err))
		}
	}()

	agentConf := agentConfig(u.serverURL)
	endpoint := agent.NewEndpoint(
		u.endpointID, server.Addr(), agentConf, nil, agent.NewMetrics(), log.NewNopLogger(),
	)
	if err = endpoint.Run(ctx); err != nil {
		return fmt.Errorf("endpoint: %w", err)
	}
	return nil
}

func agentConfig(serverURL string) *agentconfig.Config {
	return &agentconfig.Config{
		Server: agentconfig.ServerConfig{
			URL:               serverURL,
			HeartbeatInterval: time.Second,
			HeartbeatTimeout:  time.Second,
		},
		Forwarder: agentconfig.ForwarderConfig{
			Timeout: time.Second,
		},
		Admin: agentconfig.AdminConfig{
			BindAddr: "127.0.0.1:0",
		},
	}
}
