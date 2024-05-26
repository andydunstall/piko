package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/conn"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/rpc"
	"go.uber.org/zap"
)

// endpoint is responsible for registering with the Piko server then forwarding
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
			conf.Forwarder.Timeout,
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
			// connect only returns an error if it gets a non-retryable
			// response or the context is cancelled, therefore return.
			return err
		}
		defer stream.Close()

		e.logger.Debug("connected to server", zap.String("url", e.serverURL()))

		if err := stream.Monitor(
			ctx,
			e.conf.Server.HeartbeatInterval,
			e.conf.Server.HeartbeatTimeout,
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
		c, err := conn.DialWebsocket(ctx, e.serverURL(), e.conf.Auth.APIKey)
		if err == nil {
			return rpc.NewStream(c, e.rpcServer.Handler(), e.logger), nil
		}

		var retryableError *conn.RetryableError
		if !errors.As(err, &retryableError) {
			e.logger.Error(
				"failed to connect to server; non-retryable",
				zap.String("url", e.serverURL()),
				zap.Error(err),
			)
			return nil, fmt.Errorf("connect: %w", err)
		}

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
	url.Path = "/piko/v1/listener/" + e.endpointID
	if url.Scheme == "http" {
		url.Scheme = "ws"
	}
	if url.Scheme == "https" {
		url.Scheme = "wss"
	}

	return url.String()
}
