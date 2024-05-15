package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/rpc"
	"go.uber.org/zap"
)

type rpcServer struct {
	endpoint *endpoint

	rpcHandler *rpc.Handler

	logger log.Logger
}

func newRPCServer(endpoint *endpoint, logger log.Logger) *rpcServer {
	server := &rpcServer{
		endpoint:   endpoint,
		rpcHandler: rpc.NewHandler(),
		logger:     logger.WithSubsystem("rpc.server"),
	}
	server.rpcHandler.Register(rpc.TypeHeartbeat, server.Heartbeat)
	server.rpcHandler.Register(rpc.TypeProxyHTTP, server.ProxyHTTP)
	return server
}

func (s *rpcServer) Handler() *rpc.Handler {
	return s.rpcHandler
}

func (s *rpcServer) Heartbeat(b []byte) []byte {
	// Echo any received payload.
	s.logger.Debug("heartbeat rpc")
	return b
}

func (s *rpcServer) ProxyHTTP(b []byte) []byte {
	s.logger.Debug("proxy http rpc")

	var httpResp *http.Response

	httpReq, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(b)))
	if err != nil {
		s.logger.Error("proxy http rpc; failed to decode http request", zap.Error(err))

		httpResp = errorResponse(
			http.StatusInternalServerError,
			"internal error",
		)
	} else {
		httpResp = s.proxyHTTP(httpReq)
	}

	defer httpResp.Body.Close()

	var buffer bytes.Buffer
	if err := httpResp.Write(&buffer); err != nil {
		s.logger.Error("proxy http rpc; failed to encode http response", zap.Error(err))
		return nil
	}

	// TODO(andydunstall): Add header for internal errors.

	s.logger.Debug("proxy http rpc; ok", zap.String("path", httpReq.URL.Path))

	return buffer.Bytes()
}

func (s *rpcServer) proxyHTTP(r *http.Request) *http.Response {
	s.logger.Debug("proxy http rpc")

	httpResp, err := s.endpoint.ProxyHTTP(r)
	if err != nil {
		if errors.Is(err, errUpstreamTimeout) {
			s.logger.Warn("proxy http rpc; upstream timeout", zap.Error(err))

			return errorResponse(
				http.StatusGatewayTimeout,
				"upstream timeout",
			)
		} else if errors.Is(err, errUpstreamUnreachable) {
			s.logger.Warn("proxy http rpc; upstream unreachable", zap.Error(err))

			return errorResponse(
				http.StatusServiceUnavailable,
				"upstream unreachable",
			)
		} else {
			s.logger.Error("proxy http rpc; internal error", zap.Error(err))

			return errorResponse(
				http.StatusInternalServerError,
				"internal error",
			)
		}
	}
	return httpResp
}

type errorMessage struct {
	Error string `json:"error"`
}

func errorResponse(statusCode int, message string) *http.Response {
	m := &errorMessage{
		Error: message,
	}
	b, _ := json.Marshal(m)
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(b)),
	}
}
