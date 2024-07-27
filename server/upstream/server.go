package upstream

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/andydunstall/yamux"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/andydunstall/piko/pkg/auth"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/middleware"
	pikowebsocket "github.com/andydunstall/piko/pkg/websocket"
)

// Server accepts connections from upstream services.
type Server struct {
	upstreams Manager

	httpServer *http.Server

	websocketUpgrader *websocket.Upgrader

	ctx    context.Context
	cancel func()

	logger log.Logger

	Conns    map[*net.Conn]bool
	ConnsMux sync.Mutex
}

func (s *Server) connStateChange(c net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		s.ConnsMux.Lock()
		s.Conns[&c] = true
		s.ConnsMux.Unlock()
	case http.StateClosed, http.StateHijacked:
		s.ConnsMux.Lock()
		delete(s.Conns, &c)
		s.ConnsMux.Unlock()
	}
}

func NewServer(
	upstreams Manager,
	verifier auth.Verifier,
	tlsConfig *tls.Config,
	logger log.Logger,
) *Server {
	logger = logger.WithSubsystem("upstream")

	router := gin.New()
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		upstreams: upstreams,
		httpServer: &http.Server{
			Handler:   router,
			TLSConfig: tlsConfig,
			ErrorLog:  logger.StdLogger(zapcore.WarnLevel),
		},
		websocketUpgrader: &websocket.Upgrader{},
		ctx:               ctx,
		cancel:            cancel,
		logger:            logger,
		Conns:             make(map[*net.Conn]bool),
	}
	server.httpServer.ConnState = server.connStateChange

	// Recover from panics.
	router.Use(gin.CustomRecoveryWithWriter(nil, server.panicRoute))

	if verifier != nil {
		authMiddleware := middleware.NewAuth(verifier, logger)
		router.Use(authMiddleware.Verify)
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
	err := s.httpServer.Shutdown(ctx)
	// Close the context to close upstream connections.
	s.cancel()
	return err
}

// upstreamRoute handles WebSocket connections from upstream services.
func (s *Server) upstreamRoute(c *gin.Context) {
	endpointID := c.Param("endpointID")

	token, ok := c.Get(middleware.TokenContextKey)
	if ok {
		// If the token contains a set of permitted endpoints, verify the
		// target endpoint matches one of those endpoints. Otherwise if the
		// token doesn't contain any endpoints the client can access any
		// endpoint.
		endpointToken := token.(*auth.Token)
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

	s.logger.Info(
		"upstream connected",
		zap.String("endpoint-id", endpointID),
		zap.String("client-ip", c.ClientIP()),
	)
	defer s.logger.Info(
		"upstream disconnected",
		zap.String("endpoint-id", endpointID),
		zap.String("client-ip", c.ClientIP()),
	)

	ctx := s.ctx
	if ok {
		// If the token has an expiry, then we ensure we close the connection
		// to the endpoint once the token expires.
		endpointToken := token.(*auth.Token)
		if !endpointToken.Expiry.IsZero() {
			var cancel func()
			ctx, cancel = context.WithDeadline(ctx, endpointToken.Expiry)
			defer cancel()
		}
	}

	muxConfig := yamux.DefaultConfig()
	muxConfig.Logger = s.logger.StdLogger(zap.WarnLevel)
	muxConfig.LogOutput = nil
	sess, err := yamux.Server(conn, muxConfig)
	if err != nil {
		// Will not happen.
		panic("yamux server: " + err.Error())
	}
	defer sess.Close()

	upstream := NewConnUpstream(endpointID, sess)

	s.upstreams.AddConn(upstream)
	defer s.upstreams.RemoveConn(upstream)

	for {
		// The client will never open streams but block on accept to wait for
		// close or an error.
		if _, err := sess.AcceptStreamWithContext(ctx); err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if errors.Is(err, context.Canceled) {
				// Server shutdown.
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				s.logger.Info("upstream token expired")
				return
			}
			s.logger.Warn("session closed unexpectedly", zap.Error(err))
			return
		}
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
