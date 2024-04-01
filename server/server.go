package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Server is the HTTP server used for both upstream listeners and downstream
// clients.
//
// /pico is reserved for upstream listeners and management, then all other
// routes will be proxied.
type Server struct {
	httpServer *http.Server
	router     *gin.Engine

	addr string

	logger *log.Logger
}

func NewServer(addr string, logger *log.Logger) *Server {
	router := gin.New()
	// Recover from panics.
	router.Use(gin.Recovery())
	router.Use(middleware.NewLogger(
		logger.WithSubsystem("server.http.route")),
	)

	s := &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: router,
		},
		router: router,
		addr:   addr,
		logger: logger.WithSubsystem("server.http"),
	}
	s.registerRoutes()
	return s
}

func (s *Server) Serve() error {
	s.logger.Info("starting http server", zap.String("addr", s.addr))

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}
	return nil
}

// Shutdown attempts to gracefully shutdown the server by closing open
// WebSockets and waiting for pending requests to complete.
func (s *Server) Shutdown(ctx context.Context) error {
	// TODO(andydunstall): Must handle shutting down hijacked connections.
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes() {
	pico := s.router.Group("/pico/v1")
	pico.GET("/upstream", s.upstream)
	pico.GET("/health", s.health)

	// Handle not found routes, which includes all proxied endpoints.
	s.router.NoRoute(s.notFound)
}

func (s *Server) notFound(c *gin.Context) {
	// All /pico endpoints are reserved. All others are proxied.
	if strings.HasPrefix(c.Request.URL.Path, "/pico") {
		c.Status(http.StatusNotFound)
		return
	}
	s.proxy(c)
}

// proxy handles proxied requests from downstream clients.
func (s *Server) proxy(c *gin.Context) {
	c.Status(http.StatusNotImplemented)
}

// upstream handles WebSocket connections from upstream listeners.
func (s *Server) upstream(c *gin.Context) {
	c.Status(http.StatusNotImplemented)
}

func (s *Server) health(c *gin.Context) {
	c.Status(http.StatusOK)
}

func init() {
	// Disable Gin debugging.
	gin.SetMode(gin.ReleaseMode)
}
