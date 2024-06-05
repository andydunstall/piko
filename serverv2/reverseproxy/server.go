package reverseproxy

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Server struct {
	handler *ReverseProxy

	router *gin.Engine

	httpServer *http.Server

	logger log.Logger
}

func NewServer(
	upstreams UpstreamManager,
	logger log.Logger,
) *Server {
	logger = logger.WithSubsystem("reverseproxy")

	router := gin.New()
	server := &Server{
		handler: NewReverseProxy(upstreams, logger),
		router:  router,
		httpServer: &http.Server{
			Handler:  router,
			ErrorLog: logger.StdLogger(zapcore.WarnLevel),
		},
		logger: logger,
	}

	// Recover from panics.
	server.router.Use(gin.CustomRecoveryWithWriter(nil, server.panicRoute))

	server.router.Use(NewLoggerMiddleware(true, logger))

	server.registerRoutes()

	return server
}

func (s *Server) Serve(ln net.Listener) error {
	s.logger.Info("starting http server", zap.String("addr", ln.Addr().String()))
	if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
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
	s.handler.ServeHTTP(c.Writer, c.Request)
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
