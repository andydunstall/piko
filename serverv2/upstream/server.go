package upstream

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	pikowebsocket "github.com/andydunstall/piko/pkg/websocket"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.ngrok.com/muxado/v2"
)

// Server accepts connections from upstream services.
type Server struct {
	manager *Manager

	router *gin.Engine

	httpServer *http.Server

	websocketUpgrader *websocket.Upgrader

	logger log.Logger
}

func NewServer(
	manager *Manager,
	tlsConfig *tls.Config,
	logger log.Logger,
) *Server {
	router := gin.New()
	server := &Server{
		manager: manager,
		router:  router,
		httpServer: &http.Server{
			Handler:   router,
			TLSConfig: tlsConfig,
			ErrorLog:  logger.StdLogger(zapcore.WarnLevel),
		},
		websocketUpgrader: &websocket.Upgrader{},
		logger:            logger,
	}

	// Recover from panics.
	server.router.Use(gin.CustomRecoveryWithWriter(nil, server.panicRoute))

	server.registerRoutes()

	return server
}

func (s *Server) Serve(ln net.Listener) error {
	s.logger.Info(
		"starting http server",
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

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes() {
	piko := s.router.Group("/piko/v1")
	piko.GET("/upstream/:endpointID", s.wsRoute)
}

// listenerRoute handles WebSocket connections from upstream services.
func (s *Server) wsRoute(c *gin.Context) {
	wsConn, err := s.websocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade replies to the client so nothing else to do.
		s.logger.Warn("failed to upgrade websocket", zap.Error(err))
		return
	}
	conn := pikowebsocket.New(wsConn)
	defer conn.Close()

	endpointID := c.Param("endpointID")

	s.logger.Debug(
		"upstream connected",
		zap.String("endpoint-id", endpointID),
		zap.String("client-ip", c.ClientIP()),
	)
	defer s.logger.Debug(
		"upstream disconnected",
		zap.String("endpoint-id", endpointID),
		zap.String("client-ip", c.ClientIP()),
	)

	sess := muxado.NewTypedStreamSession(muxado.Server(conn, &muxado.Config{}))
	heartbeat := muxado.NewHeartbeat(
		sess,
		func(d time.Duration, timeout bool) {},
		muxado.NewHeartbeatConfig(),
	)

	upstream := NewMuxUpstream(endpointID, sess)
	s.manager.Add(upstream)
	defer s.manager.Remove(upstream)

	for {
		// The server doesn't yet accept streams, though need to keep accepting
		// to respond to heartbeats and detect close.
		_, err := heartbeat.AcceptStream()
		if err != nil {
			s.logger.Warn("accept stream", zap.Error(err))
			return
		}
	}
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
