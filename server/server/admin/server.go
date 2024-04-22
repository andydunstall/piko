package admin

import (
	"context"
	"fmt"
	"net/http"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/server/middleware"
	"github.com/andydunstall/pico/server/status"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Server is the admin HTTP server, which exposes endpoints for metrics, health
// and inspecting the node status.
type Server struct {
	addr string

	router *gin.Engine

	httpServer *http.Server

	registry *prometheus.Registry

	logger *log.Logger
}

func NewServer(
	addr string,
	registry *prometheus.Registry,
	logger *log.Logger,
) *Server {
	router := gin.New()
	server := &Server{
		addr:   addr,
		router: router,
		httpServer: &http.Server{
			Addr:    addr,
			Handler: router,
		},
		registry: registry,
		logger:   logger.WithSubsystem("admin.server"),
	}

	// Recover from panics.
	server.router.Use(gin.CustomRecovery(server.panicRoute))

	server.router.Use(middleware.NewLogger(logger))
	if registry != nil {
		router.Use(middleware.NewMetrics("admin", registry))
	}

	server.registerRoutes()

	return server
}

func (s *Server) AddStatus(route string, handler status.Handler) {
	group := s.router.Group("/status").Group(route)
	handler.Register(group)
}

func (s *Server) Serve() error {
	s.logger.Info("starting http server", zap.String("addr", s.addr))

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}
	return nil
}

// Shutdown attempts to gracefully shutdown the server by waiting for pending
// requests to complete.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes() {
	s.router.GET("/healthz", s.healthRoute)

	if s.registry != nil {
		s.router.GET("/metrics", s.metricsHandler())
	}
}

func (s *Server) healthRoute(c *gin.Context) {
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
