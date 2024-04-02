package agent

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/andydunstall/pico/agent/config"
	"github.com/andydunstall/pico/pkg/conn"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/pkg/rpc"
	"go.uber.org/zap"
)

// Listener is responsible for registering a listener with Pico for the given
// endpoint ID, then forwarding incoming requests to the given forward
// address.
type Listener struct {
	endpointID  string
	forwardAddr string

	rpcServer *rpcServer

	conf *config.Config

	logger *log.Logger
}

func NewListener(endpointID string, forwardAddr string, conf *config.Config, logger *log.Logger) *Listener {
	return &Listener{
		endpointID:  endpointID,
		forwardAddr: forwardAddr,
		rpcServer:   newRPCServer(),
		conf:        conf,
		logger:      logger.WithSubsystem("listener"),
	}
}

func (l *Listener) Run(ctx context.Context) error {
	l.logger.Info(
		"starting listener",
		zap.String("endpoint-id", l.endpointID),
		zap.String("forward-addr", l.forwardAddr),
	)

	conn, err := conn.DialWebsocket(ctx, l.serverURL())
	if err != nil {
		return fmt.Errorf("dial: %s, %w", l.serverURL(), err)
	}
	stream := rpc.NewStream(conn, l.rpcServer.Handler(), l.logger)
	defer stream.Close()

	l.logger.Debug("connected to server", zap.String("url", l.serverURL()))

	if err := l.monitorConnection(ctx, stream); err != nil {
		l.logger.Warn("disconnected", zap.Error(err))
		// TODO(andydunstall): Reconnect. Add metrics for reconnects.
		return err
	}

	return nil
}

// monitorConnection sends periodic heartbeats to ensure the connection
// to the server is ok.
//
// Returns an error if the connection is broken, or nil if ctx is cancelled.
func (l *Listener) monitorConnection(ctx context.Context, stream *rpc.Stream) error {
	ticker := time.NewTicker(
		time.Duration(l.conf.Server.HeartbeatIntervalSeconds) * time.Second,
	)
	defer ticker.Stop()

	for {
		if err := l.heartbeat(ctx, stream); err != nil {
			return fmt.Errorf("heartbeat: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (l *Listener) heartbeat(ctx context.Context, stream *rpc.Stream) error {
	heartbeatCtx, cancel := context.WithTimeout(
		ctx,
		time.Duration(l.conf.Server.HeartbeatTimeoutSeconds)*time.Second,
	)
	defer cancel()

	ts := time.Now()
	_, err := stream.RPC(heartbeatCtx, rpc.TypeHeartbeat, nil)
	if err != nil {
		// If ctx was cancelled the listener is being closed so return
		// nil.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("rpc: %w", err)
	}

	l.logger.Debug("heartbeat ok", zap.Duration("rtt", time.Since(ts)))

	return nil
}

func (l *Listener) serverURL() string {
	// Already verified URL in Config.Validate.
	url, _ := url.Parse(l.conf.Server.URL)
	url.Path = "/pico/v1/listener/" + l.endpointID
	if url.Scheme == "http" {
		url.Scheme = "ws"
	}
	if url.Scheme == "https" {
		url.Scheme = "wss"
	}

	return url.String()
}
