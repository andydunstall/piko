package agent

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/andydunstall/pico/pkg/log"
	"go.uber.org/zap"
)

var (
	errUpstreamTimeout     = errors.New("upstream timeout")
	errUpstreamUnreachable = errors.New("upstream unreachable")
)

// forwarder manages forwarding incoming HTTP requests to the configured
// upstream.
type forwarder struct {
	addr    string
	timeout time.Duration

	client *http.Client

	logger *log.Logger
}

func newForwarder(addr string, timeout time.Duration, logger *log.Logger) *forwarder {
	return &forwarder{
		addr:    addr,
		timeout: timeout,
		logger:  logger.WithSubsystem("forwarder"),
		client:  &http.Client{},
	}
}

func (f *forwarder) Forward(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()

	req = req.WithContext(ctx)

	req.URL.Scheme = "http"
	req.URL.Host = f.addr
	req.RequestURI = ""

	resp, err := f.client.Do(req)
	if err != nil {
		f.logger.Warn(
			"failed to forward request",
			zap.String("method", req.Method),
			zap.String("host", req.URL.Host),
			zap.String("path", req.URL.Path),
			zap.Int("status", resp.StatusCode),
			zap.Error(err),
		)

		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errUpstreamTimeout
		}
		return nil, errUpstreamUnreachable
	}

	// TODO(andydunstall): Add metrics and extend logging.

	f.logger.Debug(
		"forward",
		zap.String("method", req.Method),
		zap.String("host", req.URL.Host),
		zap.String("path", req.URL.Path),
		zap.Int("status", resp.StatusCode),
	)

	return resp, nil
}
