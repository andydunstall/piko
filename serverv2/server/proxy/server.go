package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/andydunstall/pico/pkg/conn"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/pkg/rpc"
	"github.com/andydunstall/pico/serverv2/proxy"
	"github.com/andydunstall/pico/serverv2/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Server is the HTTP server for the proxy, which is used for both upstream
// listeners and downstream clients.
//
// /pico is reserved for upstream listeners, all other routes will be proxied.
type Server struct {
	addr string

	router *gin.Engine

	httpServer *http.Server
	rpcServer  *rpcServer

	websocketUpgrader *websocket.Upgrader

	proxy *proxy.Proxy

	shutdownCtx    context.Context
	shutdownCancel func()

	logger *log.Logger
}

func NewServer(
	addr string,
	proxy *proxy.Proxy,
	registry *prometheus.Registry,
	logger *log.Logger,
) *Server {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

	router := gin.New()
	server := &Server{
		addr:   addr,
		router: router,
		httpServer: &http.Server{
			Addr:    addr,
			Handler: router,
		},
		rpcServer:         newRPCServer(),
		websocketUpgrader: &websocket.Upgrader{},
		shutdownCtx:       shutdownCtx,
		shutdownCancel:    shutdownCancel,
		proxy:             proxy,
		logger:            logger.WithSubsystem("proxy.server"),
	}

	// Recover from panics.
	server.router.Use(gin.CustomRecovery(server.panicRoute))

	server.router.Use(middleware.NewLogger(logger))
	if registry != nil {
		router.Use(middleware.NewMetrics("proxy", registry))
	}

	server.registerRoutes()

	return server
}

func (s *Server) Serve() error {
	s.logger.Info("starting http server", zap.String("addr", s.addr))

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes() {
	pico := s.router.Group("/pico/v1")
	pico.GET("/listener/:endpointID", s.listenerRoute)

	// Handle not found routes, which includes all proxied endpoints.
	s.router.NoRoute(s.notFoundRoute)
}

// listenerRoute handles WebSocket connections from upstream listeners.
func (s *Server) listenerRoute(c *gin.Context) {
	endpointID := c.Param("endpointID")

	wsConn, err := s.websocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade replies to the client so nothing else to do.
		s.logger.Warn("failed to upgrade websocket", zap.Error(err))
		return
	}
	stream := rpc.NewStream(
		conn.NewWebsocketConn(wsConn),
		s.rpcServer.Handler(),
		s.logger,
	)
	defer stream.Close()

	s.logger.Debug(
		"listener connected",
		zap.String("endpoint-id", endpointID),
		zap.String("client-ip", c.ClientIP()),
	)

	s.proxy.AddUpstream(endpointID, stream)
	defer s.proxy.RemoveUpstream(endpointID, stream)

	if err := stream.Monitor(
		s.shutdownCtx,
		// TODO(andydunstall): Configurable.
		time.Second*10,
		time.Second*10,
	); err != nil {
		s.logger.Debug("listener disconnected", zap.Error(err))
	}
}

func (s *Server) proxyRoute(c *gin.Context) {
	c.Status(http.StatusNotImplemented)
}

func (s *Server) notFoundRoute(c *gin.Context) {
	// All /pico endpoints are reserved. All others are proxied.
	if strings.HasPrefix(c.Request.URL.Path, "/pico") {
		c.Status(http.StatusNotFound)
		return
	}
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
