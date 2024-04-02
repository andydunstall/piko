package server

import (
	"context"
	"fmt"
	"time"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/pkg/rpc"
	"github.com/andydunstall/pico/server/config"
	"go.uber.org/zap"
)

// listener represents a connected upstream listener.
type listener struct {
	endpointID string
	stream     *rpc.Stream
	conf       config.UpstreamConfig
	logger     *log.Logger
}

func newListener(endpointID string, stream *rpc.Stream, conf config.UpstreamConfig, logger *log.Logger) *listener {
	return &listener{
		endpointID: endpointID,
		stream:     stream,
		conf:       conf,
		logger:     logger.WithSubsystem("listener"),
	}
}

// Monitor sends periodic heartbeats to the listener to verify the connection
// is healthy.
//
// Returns an error if the connection is broken, or nil if ctx is cancelled.
func (l *listener) Monitor(ctx context.Context) error {
	ticker := time.NewTicker(
		time.Duration(l.conf.HeartbeatIntervalSeconds) * time.Second,
	)
	defer ticker.Stop()

	for {
		if err := l.heartbeat(ctx, l.stream); err != nil {
			return fmt.Errorf("heartbeat: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (l *listener) heartbeat(ctx context.Context, stream *rpc.Stream) error {
	heartbeatCtx, cancel := context.WithTimeout(
		ctx,
		time.Duration(l.conf.HeartbeatTimeoutSeconds)*time.Second,
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
