package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/config"
	"github.com/andydunstall/pico/server/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type Proxy interface {
	Request(ctx context.Context, r *http.Request) *http.Response
}

// Server is the HTTP server for the proxy, which proxies all incoming
// requests.
type Server struct {
	ln net.Listener

	router *gin.Engine

	httpServer *http.Server

	proxy Proxy

	shutdownCtx    context.Context
	shutdownCancel func()

	conf *config.ProxyConfig

	logger log.Logger
}

func NewServer(
	ln net.Listener,
	proxy Proxy,
	conf *config.ProxyConfig,
	registry *prometheus.Registry,
	logger log.Logger,
) *Server {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	router := gin.New()
	server := &Server{
		ln:     ln,
		router: router,
		httpServer: &http.Server{
			Addr:    ln.Addr().String(),
			Handler: router,
		},
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
		proxy:          proxy,
		conf:           conf,
		logger:         logger.WithSubsystem("proxy.server"),
	}

	// Recover from panics.
	server.router.Use(gin.CustomRecoveryWithWriter(nil, server.panicRoute))

	server.router.Use(middleware.NewLogger(logger))
	if registry != nil {
		router.Use(middleware.NewMetrics("proxy", registry))
	}

	server.registerRoutes()

	return server
}

func (s *Server) Serve() error {
	s.logger.Info("starting http server", zap.String("addr", s.ln.Addr().String()))

	if err := s.httpServer.Serve(s.ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes() {
	// Handle not found routes, which includes all proxied endpoints.
	s.router.NoRoute(s.notFoundRoute)
}

// proxyRoute handles proxied requests from proxy clients.
func (s *Server) proxyRoute(c *gin.Context) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(s.conf.GatewayTimeout)*time.Second,
	)
	defer cancel()

	resp := s.proxy.Request(ctx, c.Request)
	// Write the response status, headers and body.
	for k, v := range resp.Header {
		c.Writer.Header()[k] = v
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		s.logger.Warn("failed to write response", zap.Error(err))
	}
}

func (s *Server) notFoundRoute(c *gin.Context) {
	s.proxyRoute(c)
}

func (s *Server) panicRoute(c *gin.Context, err any) {
	s.logger.Error(
		"handler panic",
		zap.String("path", c.FullPath()),
		zap.Any("err", err),
	)
	c.AbortWithStatus(http.StatusInternalServerError)
}

func init() {
	// Disable Gin debug logs.
	gin.SetMode(gin.ReleaseMode)
}
