package agent

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var (
	errUpstreamTimeout     = errors.New("upstream timeout")
	errUpstreamUnreachable = errors.New("upstream unreachable")
)

// forwarder manages forwarding incoming HTTP requests to the configured
// upstream.
type forwarder struct {
	endpointID string
	addr       string
	timeout    time.Duration

	client *http.Client

	metrics *Metrics

	logger log.Logger
}

func newForwarder(
	endpointID string,
	addr string,
	timeout time.Duration,
	metrics *Metrics,
	logger log.Logger,
) *forwarder {
	return &forwarder{
		endpointID: endpointID,
		addr:       addr,
		timeout:    timeout,
		client:     &http.Client{},
		metrics:    metrics,
		logger:     logger.WithSubsystem("forwarder"),
	}
}

func (f *forwarder) Forward(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()

	req = req.WithContext(ctx)

	req.URL.Scheme = "http"
	req.URL.Host = f.addr
	req.RequestURI = ""

	start := time.Now()

	resp, err := f.client.Do(req)
	if err != nil {
		f.logger.Warn(
			"failed to forward request",
			zap.String("method", req.Method),
			zap.String("host", req.URL.Host),
			zap.String("path", req.URL.Path),
			zap.Error(err),
		)

		f.metrics.ForwardErrorsTotal.With(prometheus.Labels{
			"endpoint_id": f.endpointID,
		}).Inc()

		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errUpstreamTimeout
		}
		return nil, errUpstreamUnreachable
	}

	f.metrics.ForwardRequestsTotal.With(prometheus.Labels{
		"method":      req.Method,
		"status":      strconv.Itoa(resp.StatusCode),
		"endpoint_id": f.endpointID,
	}).Inc()
	f.metrics.ForwardRequestLatency.With(prometheus.Labels{
		"status":      strconv.Itoa(resp.StatusCode),
		"endpoint_id": f.endpointID,
	}).Observe(float64(time.Since(start).Milliseconds()) / 1000)

	f.logger.Debug(
		"forward",
		zap.String("method", req.Method),
		zap.String("host", req.URL.Host),
		zap.String("path", req.URL.Path),
		zap.Int("status", resp.StatusCode),
	)

	return resp, nil
}
