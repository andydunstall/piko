package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/andydunstall/pico/pkg/log"
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

	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: router,
		},
		router: router,
		addr:   addr,
		logger: logger.WithSubsystem("server.http"),
	}
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

func init() {
	// Disable Gin debugging.
	gin.SetMode(gin.ReleaseMode)
}
