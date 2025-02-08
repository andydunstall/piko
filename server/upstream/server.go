package upstream

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
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
	"github.com/andydunstall/piko/server/cluster"
	"github.com/andydunstall/piko/server/config"
)

// Server accepts connections from upstream services.
type Server struct {
	upstreams Manager

	sessions   map[*yamux.Session]struct{}
	sessionsMu sync.Mutex

	httpServer *http.Server

	websocketUpgrader *websocket.Upgrader

	ctx    context.Context
	cancel func()

	cluster *cluster.State

	config config.UpstreamConfig

	logger log.Logger
}

func NewServer(
	upstreams Manager,
	verifier auth.Verifier,
	tlsConfig *tls.Config,
	cluster *cluster.State,
	config config.UpstreamConfig,
	logger log.Logger,
) *Server {
	logger = logger.WithSubsystem("upstream")

	router := gin.New()
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		upstreams: upstreams,
		sessions:  make(map[*yamux.Session]struct{}),
		httpServer: &http.Server{
			Handler:   router,
			TLSConfig: tlsConfig,
			ErrorLog:  logger.StdLogger(zapcore.WarnLevel),
		},
		websocketUpgrader: &websocket.Upgrader{},
		ctx:               ctx,
		cancel:            cancel,
		cluster:           cluster,
		config:            config,
		logger:            logger,
	}

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

func (s *Server) Rebalance() {
	if len(s.cluster.Nodes()) <= 1 {
		s.logger.Debug("rebalance; skip; no other nodes")
		return
	}

	localConns := s.openSessions()
	if localConns == 0 || localConns < int(s.config.Rebalance.MinConns) {
		s.logger.Debug(
			"rebalance; skip; too few conns",
			zap.Int("local_conns", localConns),
		)
		return
	}

	avgConns := s.cluster.AvgConns()
	balance := float64(localConns-avgConns) / float64(avgConns)
	if balance < s.config.Rebalance.Threshold {
		s.logger.Debug(
			"rebalance; skip; below threshold",
			zap.String("balance", fmt.Sprintf("%.2f", balance)),
			zap.Float64("threshold", s.config.Rebalance.Threshold),
			zap.Int("local_conns", localConns),
			zap.Int("avg_conns", avgConns),
		)
		return
	}

	// Shed up to the shed rate (as Rebalance is called every second).
	shedding := float64(localConns) * balance
	if shedding > float64(avgConns)*s.config.Rebalance.ShedRate {
		shedding = math.Ceil(float64(avgConns) * s.config.Rebalance.ShedRate)
	}

	s.logger.Info(
		"rebalance; shedding connections",
		zap.Int("shedding", int(shedding)),
		zap.String("balance", fmt.Sprintf("%.2f", balance)),
		zap.Float64("threshold", s.config.Rebalance.Threshold),
		zap.Int("local_conns", localConns),
		zap.Int("avg_conns", avgConns),
	)
	s.shedSessions(int(shedding))
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

	s.addSession(sess)
	defer s.removeSession(sess)

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
			if errors.Is(err, yamux.ErrSessionShutdown) {
				return
			}
			s.logger.Warn("session closed unexpectedly", zap.Error(err))
			return
		}
	}
}

func (s *Server) addSession(sess *yamux.Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	s.sessions[sess] = struct{}{}
}

func (s *Server) removeSession(sess *yamux.Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	delete(s.sessions, sess)
}

func (s *Server) openSessions() int {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	return len(s.sessions)
}

func (s *Server) shedSessions(n int) {
	// Don't hold the mutex during close as Close could block.
	s.sessionsMu.Lock()
	var shedding []*yamux.Session
	for sess := range s.sessions {
		shedding = append(shedding, sess)
		if len(shedding) >= n {
			break
		}
	}
	s.sessionsMu.Unlock()

	// Note don't update s.sessions, as the session will automatically be
	// removed when it's closed.
	for _, sess := range shedding {
		sess.Close()
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
