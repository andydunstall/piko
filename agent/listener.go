package agent

import (
	"context"
	"fmt"
	"net/http"
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

	forwarder *forwarder

	rpcServer *rpcServer

	conf *config.Config

	logger log.Logger
}

func NewListener(endpointID string, forwardAddr string, conf *config.Config, logger log.Logger) *Listener {
	l := &Listener{
		endpointID:  endpointID,
		forwardAddr: forwardAddr,
		forwarder: newForwarder(
			forwardAddr,
			time.Duration(conf.Forwarder.TimeoutSeconds)*time.Second,
			logger,
		),
		conf:   conf,
		logger: logger.WithSubsystem("listener"),
	}
	l.rpcServer = newRPCServer(l, logger)
	return l
}

func (l *Listener) Run(ctx context.Context) error {
	l.logger.Info(
		"starting listener",
		zap.String("endpoint-id", l.endpointID),
		zap.String("forward-addr", l.forwardAddr),
	)

	for {
		stream, err := l.connect(ctx)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer stream.Close()

		l.logger.Debug("connected to server", zap.String("url", l.serverURL()))

		if err := stream.Monitor(
			ctx,
			time.Duration(l.conf.Server.HeartbeatIntervalSeconds)*time.Second,
			time.Duration(l.conf.Server.HeartbeatTimeoutSeconds)*time.Second,
		); err != nil {
			if ctx.Err() != nil {
				// Shutdown.
				return ctx.Err()
			}

			// Reconnect.
			l.logger.Warn("disconnected", zap.Error(err))
		}
	}
}

func (l *Listener) ProxyHTTP(r *http.Request) (*http.Response, error) {
	return l.forwarder.Forward(r)
}

func (l *Listener) connect(ctx context.Context) (*rpc.Stream, error) {
	backoff := time.Second
	for {
		conn, err := conn.DialWebsocket(ctx, l.serverURL())
		if err == nil {
			return rpc.NewStream(conn, l.rpcServer.Handler(), l.logger), nil
		}

		l.logger.Warn(
			"failed to connect to server; retrying",
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)

		select {
		case <-time.After(backoff):
			backoff *= 2
			if backoff > time.Second*30 {
				backoff = time.Second * 30
			}
			continue
		case <-ctx.Done():
			return nil, err
		}
	}
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
