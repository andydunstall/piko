package agent

import (
	"context"

	"github.com/andydunstall/pico/pkg/log"
	"go.uber.org/zap"
)

// Listener is responsible for registering a listener with Pico for the given
// endpoint ID, then forwarding incoming requests to the given forward
// address.
type Listener struct {
	endpointID  string
	forwardAddr string

	logger *log.Logger
}

func NewListener(endpointID string, forwardAddr string, logger *log.Logger) *Listener {
	return &Listener{
		endpointID:  endpointID,
		forwardAddr: forwardAddr,
		logger:      logger.WithSubsystem("listener"),
	}
}

func (l *Listener) Run(ctx context.Context) error {
	l.logger.Info(
		"starting listener",
		zap.String("endpoint-id", l.endpointID),
		zap.String("forward-addr", l.forwardAddr),
	)
	return nil
}
