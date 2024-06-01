package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/protocol"
	pikowebsocket "github.com/andydunstall/piko/pkg/websocket"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.ngrok.com/muxado/v2"
)

// Server accepts connections from upstream services.
type Server struct {
	router *gin.Engine

	httpServer *http.Server

	websocketUpgrader *websocket.Upgrader

	logger log.Logger
}

func NewServer(
	tlsConfig *tls.Config,
	logger log.Logger,
) *Server {
	router := gin.New()
	server := &Server{
		router: router,
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
	piko.GET("/upstream/ws", s.wsRoute)
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

	s.logger.Debug(
		"listener connected",
		zap.String("client-ip", c.ClientIP()),
	)
	defer s.logger.Debug(
		"listener disconnected",
		zap.String("client-ip", c.ClientIP()),
	)

	sess := muxado.NewTypedStreamSession(muxado.Server(conn, &muxado.Config{}))
	heartbeat := muxado.NewHeartbeat(
		sess,
		func(d time.Duration, timeout bool) {},
		muxado.NewHeartbeatConfig(),
	)

	for {
		stream, err := heartbeat.AcceptTypedStream()
		if err != nil {
			s.logger.Warn("accept stream", zap.Error(err))
			return
		}
		go func() {
			if err := s.handleStream(stream); err != nil {
				s.logger.Warn("handle stream", zap.Error(err))
			}
		}()
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

func (s *Server) handleStream(stream muxado.TypedStream) error {
	switch protocol.RPCType(stream.StreamType()) {
	case protocol.RPCTypeListen:
		if err := s.handleListenRequest(stream); err != nil {
			return fmt.Errorf("listen request: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported rpc type: %d", stream.StreamType())
	}
}

func (s *Server) handleListenRequest(stream muxado.TypedStream) error {
	var req protocol.ListenRequest
	if err := json.NewDecoder(stream).Decode(&req); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}

	resp := protocol.ListenResponse(req)
	if err := json.NewEncoder(stream).Encode(resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	s.logger.Info(
		"registered listener",
		zap.String("endpoint-id", req.EndpointID),
	)

	// TODO(andydunstall): Register.

	return nil
}

func init() {
	// Disable Gin debug logs.
	gin.SetMode(gin.ReleaseMode)
}
