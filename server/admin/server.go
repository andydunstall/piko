package admin

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/status"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Server is the admin HTTP server, which exposes endpoints for metrics, health
// and inspecting the node status.
type Server struct {
	registry *prometheus.Registry

	httpServer *http.Server

	router *gin.Engine

	logger log.Logger
}

func NewServer(
	registry *prometheus.Registry,
	tlsConfig *tls.Config,
	logger log.Logger,
) *Server {
	logger = logger.WithSubsystem("admin")

	router := gin.New()
	server := &Server{
		registry: registry,
		httpServer: &http.Server{
			Handler:   router,
			TLSConfig: tlsConfig,
			ErrorLog:  logger.StdLogger(zapcore.WarnLevel),
		},
		router: router,
		logger: logger,
	}

	// Recover from panics.
	router.Use(gin.CustomRecoveryWithWriter(nil, server.panicRoute))

	server.registerRoutes(router)

	return server
}

func (s *Server) Serve(ln net.Listener) error {
	s.logger.Info(
		"starting admin server",
		zap.String("addr", ln.Addr().String()),
	)

	var err error
	if s.httpServer.TLSConfig != nil {
		err = s.httpServer.ServeTLS(ln, "", "")
	} else {
		err = s.httpServer.Serve(ln)
	}

	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}
	return nil
}

// Shutdown attempts to gracefully shutdown the server by waiting for pending
// requests to complete.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) AddStatus(route string, handler status.Handler) {
	group := s.router.Group("/status").Group(route)
	handler.Register(group)
}

func (s *Server) registerRoutes(router *gin.Engine) {
	router.GET("/health", s.healthRoute)

	if s.registry != nil {
		router.GET("/metrics", s.metricsHandler())
	}
}

func (s *Server) healthRoute(c *gin.Context) {
	c.Status(http.StatusOK)
}

func (s *Server) panicRoute(c *gin.Context, err any) {
	s.logger.Error(
		"handler panic",
		zap.String("path", c.FullPath()),
		zap.Any("err", err),
	)
	c.AbortWithStatus(http.StatusInternalServerError)
}

func (s *Server) metricsHandler() gin.HandlerFunc {
	h := promhttp.HandlerFor(
		s.registry,
		promhttp.HandlerOpts{Registry: s.registry},
	)
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func init() {
	// Disable Gin debug logs.
	gin.SetMode(gin.ReleaseMode)
}
