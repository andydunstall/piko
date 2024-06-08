package upstream

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/mux"
	pikowebsocket "github.com/andydunstall/piko/pkg/websocket"
	"github.com/andydunstall/piko/server/auth"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Server accepts connections from upstream services.
type Server struct {
	upstreams Manager

	httpServer *http.Server

	websocketUpgrader *websocket.Upgrader

	logger log.Logger
}

func NewServer(
	upstreams Manager,
	verifier auth.Verifier,
	tlsConfig *tls.Config,
	logger log.Logger,
) *Server {
	logger = logger.WithSubsystem("admin")

	router := gin.New()
	server := &Server{
		upstreams: upstreams,
		httpServer: &http.Server{
			Handler:   router,
			TLSConfig: tlsConfig,
			ErrorLog:  logger.StdLogger(zapcore.WarnLevel),
		},
		websocketUpgrader: &websocket.Upgrader{},
		logger:            logger,
	}

	// Recover from panics.
	router.Use(gin.CustomRecoveryWithWriter(nil, server.panicRoute))

	if verifier != nil {
		authMiddleware := NewAuthMiddleware(verifier, logger)
		router.Use(authMiddleware.VerifyEndpointToken)
	}

	server.registerRoutes(router)

	return server
}

func (s *Server) Serve(ln net.Listener) error {
	s.logger.Info(
		"starting upstream server",
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

// upstreamRoute handles WebSocket connections from upstream services.
func (s *Server) upstreamRoute(c *gin.Context) {
	endpointID := c.Param("endpointID")

	token, ok := c.Get(TokenContextKey)
	if ok {
		endpointToken := token.(*auth.EndpointToken)
		if !endpointToken.EndpointPermitted(endpointID) {
			s.logger.Warn(
				"endpoint not permitted",
				zap.Strings("token-endpoints", endpointToken.Endpoints),
				zap.String("endpoint-id", endpointID),
			)
			c.JSON(
				http.StatusUnauthorized,
				gin.H{"error": "endpoint not permitted"},
			)
			return
		}
	}

	wsConn, err := s.websocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade replies to the client so nothing else to do.
		s.logger.Warn("failed to upgrade websocket", zap.Error(err))
		return
	}
	conn := pikowebsocket.New(wsConn)
	defer conn.Close()

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

	ctx := context.Background()
	if ok {
		// If the token has an expiry, then we ensure we close the connection
		// to the endpoint once the token expires.
		endpointToken := token.(*auth.EndpointToken)
		if !endpointToken.Expiry.IsZero() {
			var cancel func()
			ctx, cancel = context.WithDeadline(ctx, endpointToken.Expiry)
			defer cancel()
		}
	}

	sess := mux.OpenServer(conn)
	upstream := NewConnUpstream(endpointID, sess)

	s.upstreams.AddConn(upstream)
	defer s.upstreams.RemoveConn(upstream)

	closedCh := make(chan struct{})
	go func() {
		if err := sess.Wait(); err != nil {
			s.logger.Warn("session closed", zap.Error(err))
		}
		close(closedCh)
	}()

	select {
	case <-ctx.Done():
		s.logger.Warn("token expired")
	case <-closedCh:
	}
}

func (s *Server) registerRoutes(router *gin.Engine) {
	piko := router.Group("/piko/v1")
	piko.GET("/upstream/:endpointID", s.upstreamRoute)
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
