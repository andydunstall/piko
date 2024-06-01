package proxy

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/serverv2/upstream"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Server struct {
	server *http.Server

	logger log.Logger
}

func NewServer(manager *upstream.Manager, logger log.Logger) *Server {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// TODO(andydunstall): Currently connecting to any available
			// upstream.
			return manager.Dial()
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// TODO(andydunstall): Configure timeouts, access log, ...
	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(r *httputil.ProxyRequest) {
			u, _ := url.Parse("http://localhost:9999")
			r.SetURL(u)
		},
		ErrorLog: logger.StdLogger(zapcore.WarnLevel),
	}
	return &Server{
		server: &http.Server{
			Handler:  proxy,
			ErrorLog: logger.StdLogger(zapcore.WarnLevel),
		},
		logger: logger,
	}
}

// Serve serves connections on the listener.
func (s *Server) Serve(ln net.Listener) error {
	s.logger.Info(
		"starting http server",
		zap.String("addr", ln.Addr().String()),
	)
	return s.server.Serve(ln)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
