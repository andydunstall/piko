package agent

import (
	"context"
	"fmt"
	"net/url"

	"github.com/andydunstall/pico/agent/config"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Listener is responsible for registering a listener with Pico for the given
// endpoint ID, then forwarding incoming requests to the given forward
// address.
type Listener struct {
	endpointID  string
	forwardAddr string

	conf *config.Config

	logger *log.Logger
}

func NewListener(endpointID string, forwardAddr string, conf *config.Config, logger *log.Logger) *Listener {
	return &Listener{
		endpointID:  endpointID,
		forwardAddr: forwardAddr,
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

	// Already verified URL in Config.Validate.
	url, _ := url.Parse(l.conf.Server.URL)
	url.Path = "/pico/v1/listener/" + l.endpointID
	if url.Scheme == "http" {
		url.Scheme = "ws"
	}
	if url.Scheme == "https" {
		url.Scheme = "wss"
	}

	wsConn, _, err := websocket.DefaultDialer.DialContext(
		ctx, url.String(), nil,
	)
	if err != nil {
		return fmt.Errorf("dial: %s, %w", url.String(), err)
	}
	defer wsConn.Close()

	l.logger.Debug("connected to server", zap.String("url", url.String()))

	return nil
}
