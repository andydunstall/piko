package reverseproxy

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/middleware"
)

type Server struct {
	proxy *ReverseProxy

	router *gin.Engine

	httpServer *http.Server

	logger log.Logger
}

func NewServer(
	conf config.ListenerConfig,
	metrics *middleware.LabeledMetrics,
	logger log.Logger,
) *Server {
	logger = logger.WithSubsystem("proxy.http")
	logger = logger.With(zap.String("endpoint-id", conf.EndpointID))

	router := gin.New()
	s := &Server{
		proxy:  NewReverseProxy(conf, logger),
		router: router,
		httpServer: &http.Server{
			Handler:  router,
			ErrorLog: logger.StdLogger(zapcore.WarnLevel),
		},
		logger: logger,
	}

	// Recover from panics.
	s.router.Use(gin.CustomRecoveryWithWriter(nil, s.panicRoute))

	s.router.Use(middleware.NewLogger(conf.AccessLog, logger))

	if metrics != nil {
		router.Use(metrics.Handler(conf.EndpointID))
	}

	s.router.NoRoute(s.proxyRoute)

	return s
}

func (s *Server) Serve(ln net.Listener) error {
	s.logger.Info("starting reverse proxy")

	if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) proxyRoute(c *gin.Context) {
	s.proxy.ServeHTTP(c.Writer, c.Request)
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
