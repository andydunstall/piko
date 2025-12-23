package admin

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dragonflydb/piko/pkg/auth"
	"github.com/dragonflydb/piko/pkg/log"
	"github.com/dragonflydb/piko/pkg/middleware"
	"github.com/dragonflydb/piko/server/cluster"
	"github.com/dragonflydb/piko/server/status"
)

// Server is the admin HTTP server, which exposes endpoints for metrics, health
// and inspecting the node status.
type Server struct {
	clusterState *cluster.State

	ready *atomic.Bool

	registry *prometheus.Registry

	proxy *ReverseProxy

	httpServer *http.Server

	router *gin.Engine

	logger log.Logger
}

func NewServer(
	clusterState *cluster.State,
	registry *prometheus.Registry,
	verifier *auth.MultiTenantVerifier,
	tlsConfig *tls.Config,
	logger log.Logger,
) *Server {
	logger = logger.WithSubsystem("admin")

	router := gin.New()
	server := &Server{
		clusterState: clusterState,
		ready:        atomic.NewBool(false),
		registry:     registry,
		proxy:        NewReverseProxy(logger),
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

	if verifier != nil {
		authMiddleware := middleware.NewAuth(verifier, logger)
		router.Use(authMiddleware.Verify)
	}

	if clusterState != nil {
		router.Use(server.forwardInterceptor)
	}

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

func (s *Server) SetReady(ready bool) {
	s.ready.Store(ready)
}

func (s *Server) registerRoutes(router *gin.Engine) {
	router.GET("/health", s.healthRoute)
	router.GET("/ready", s.readyRoute)

	if s.registry != nil {
		router.GET("/metrics", s.metricsHandler())
	}

	// From https://github.com/gin-contrib/pprof/blob/934af36b21728278339704005bcef2eec1375091/pprof.go#L32.
	pprofGroup := s.router.Group("/debug/pprof")
	pprofGroup.GET("/", gin.WrapF(pprof.Index))
	pprofGroup.GET("/cmdline", gin.WrapF(pprof.Cmdline))
	pprofGroup.GET("/profile", gin.WrapF(pprof.Profile))
	pprofGroup.POST("/symbol", gin.WrapF(pprof.Symbol))
	pprofGroup.GET("/symbol", gin.WrapF(pprof.Symbol))
	pprofGroup.GET("/trace", gin.WrapF(pprof.Trace))
	pprofGroup.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
	pprofGroup.GET("/block", gin.WrapH(pprof.Handler("block")))
	pprofGroup.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	pprofGroup.GET("/heap", gin.WrapH(pprof.Handler("heap")))
	pprofGroup.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
	pprofGroup.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
}

func (s *Server) healthRoute(c *gin.Context) {
	c.Status(http.StatusOK)
}

func (s *Server) readyRoute(c *gin.Context) {
	if !s.ready.Load() {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	c.Status(http.StatusOK)
}

// forwardInterceptor intercepts all admin requests. If the request has a
// 'forward' query, the request is forwarded to the node with the requested ID.
func (s *Server) forwardInterceptor(c *gin.Context) {
	forward, ok := c.GetQuery("forward")
	if !ok || forward == s.clusterState.LocalID() {
		// No forward configuration so handle locally.
		c.Next()
		return
	}

	node, ok := s.clusterState.Node(forward)
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	ctx := context.WithValue(c.Request.Context(), hostContextKey, node.AdminAddr)
	r := c.Request.WithContext(ctx)

	s.proxy.ServeHTTP(c.Writer, r)

	// Abort to avoid going to the next handler.
	c.Abort()
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
