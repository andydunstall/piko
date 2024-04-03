package agent

import (
	"bufio"
	"bytes"
	"errors"
	"net/http"

	"github.com/andydunstall/pico/api"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/pkg/rpc"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type rpcServer struct {
	listener *Listener

	rpcHandler *rpc.Handler

	logger *log.Logger
}

func newRPCServer(listener *Listener, logger *log.Logger) *rpcServer {
	server := &rpcServer{
		listener:   listener,
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

	var protoReq api.ProxyHttpReq
	if err := proto.Unmarshal(b, &protoReq); err != nil {
		s.logger.Error("proxy http rpc; failed to decode proto request", zap.Error(err))

		return s.proxyHTTPError(
			api.ProxyHttpStatus_INTERNAL_ERROR,
			"decode proto request",
		)
	}

	httpReq, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(protoReq.HttpReq)))
	if err != nil {
		s.logger.Error("proxy http rpc; failed to decode http request", zap.Error(err))

		return s.proxyHTTPError(
			api.ProxyHttpStatus_INTERNAL_ERROR,
			"decode http request",
		)
	}

	httpResp, err := s.listener.ProxyHTTP(httpReq)
	if err != nil {
		if errors.Is(err, errUpstreamTimeout) {
			s.logger.Error("proxy http rpc; upstream timeout", zap.Error(err))

			return s.proxyHTTPError(
				api.ProxyHttpStatus_UPSTREAM_TIMEOUT,
				"upstream timeout",
			)
		} else if errors.Is(err, errUpstreamUnreachable) {
			s.logger.Error("proxy http rpc; upstream unreachable", zap.Error(err))

			return s.proxyHTTPError(
				api.ProxyHttpStatus_UPSTREAM_UNREACHABLE,
				"upstream unreachable",
			)
		} else {
			s.logger.Error("proxy http rpc; internal error", zap.Error(err))

			return s.proxyHTTPError(
				api.ProxyHttpStatus_INTERNAL_ERROR,
				err.Error(),
			)
		}
	}

	defer httpResp.Body.Close()

	var buffer bytes.Buffer
	if err := httpResp.Write(&buffer); err != nil {
		s.logger.Error("proxy http rpc; failed to encode http response", zap.Error(err))

		return s.proxyHTTPError(
			api.ProxyHttpStatus_INTERNAL_ERROR,
			"failed to encode http response",
		)
	}

	protoResp := &api.ProxyHttpResp{
		HttpResp: buffer.Bytes(),
	}
	payload, err := proto.Marshal(protoResp)
	if err != nil {
		// This should never happen, so the only remaining action is to
		// panic.
		panic("failed to encode proto response: " + err.Error())
	}

	s.logger.Debug("proxy http rpc; ok", zap.String("path", httpReq.URL.Path))

	return payload
}

func (s *rpcServer) proxyHTTPError(status api.ProxyHttpStatus, message string) []byte {
	resp := &api.ProxyHttpResp{
		Error: &api.ProxyHttpError{
			Status:  status,
			Message: message,
		},
	}
	payload, err := proto.Marshal(resp)
	if err != nil {
		// This should never happen, so the only remaining action is to
		// panic.
		panic("failed to encode proto response: " + err.Error())
	}
	return payload
}
