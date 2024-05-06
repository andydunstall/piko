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

// endpoint is responsible for registering with the Pico server then forwarding
// incoming requests to the forward address.
type endpoint struct {
	endpointID  string
	forwardAddr string

	forwarder *forwarder

	rpcServer *rpcServer

	conf *config.Config

	metrics *Metrics

	logger log.Logger
}

func newEndpoint(
	endpointID string,
	forwardAddr string,
	conf *config.Config,
	metrics *Metrics,
	logger log.Logger,
) *endpoint {
	e := &endpoint{
		endpointID:  endpointID,
		forwardAddr: forwardAddr,
		forwarder: newForwarder(
			endpointID,
			forwardAddr,
			time.Duration(conf.Forwarder.Timeout)*time.Second,
			metrics,
			logger,
		),
		conf:    conf,
		metrics: metrics,
		logger:  logger.WithSubsystem("listener"),
	}
	e.rpcServer = newRPCServer(e, logger)
	return e
}

func (e *endpoint) Run(ctx context.Context) error {
	e.logger.Info(
		"starting endpoint",
		zap.String("endpoint-id", e.endpointID),
		zap.String("forward-addr", e.forwardAddr),
	)

	for {
		stream, err := e.connect(ctx)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer stream.Close()

		e.logger.Debug("connected to server", zap.String("url", e.serverURL()))

		if err := stream.Monitor(
			ctx,
			time.Duration(e.conf.Server.HeartbeatInterval)*time.Second,
			time.Duration(e.conf.Server.HeartbeatTimeout)*time.Second,
		); err != nil {
			if ctx.Err() != nil {
				// Shutdown.
				return ctx.Err()
			}

			// Reconnect.
			e.logger.Warn("disconnected", zap.Error(err))
		}
	}
}

func (e *endpoint) ProxyHTTP(r *http.Request) (*http.Response, error) {
	return e.forwarder.Forward(r)
}

func (e *endpoint) connect(ctx context.Context) (rpc.Stream, error) {
	backoff := time.Second
	for {
		conn, err := conn.DialWebsocket(ctx, e.serverURL(), e.conf.Auth.APIKey)
		if err == nil {
			return rpc.NewStream(conn, e.rpcServer.Handler(), e.logger), nil
		}

		// TODO(andydunstall): Handle non-retryable errors like 401.

		e.logger.Warn(
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

func (e *endpoint) serverURL() string {
	// Already verified URL in Config.Validate.
	url, _ := url.Parse(e.conf.Server.URL)
	url.Path = "/pico/v1/listener/" + e.endpointID
	if url.Scheme == "http" {
		url.Scheme = "ws"
	}
	if url.Scheme == "https" {
		url.Scheme = "wss"
	}

	return url.String()
}
