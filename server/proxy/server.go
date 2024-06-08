package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/middleware"
	"github.com/andydunstall/piko/server/config"
	"github.com/andydunstall/piko/server/upstream"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Server struct {
	proxy *ReverseProxy

	httpServer *http.Server

	logger log.Logger
}

func NewServer(
	upstreams upstream.Manager,
	proxyConfig config.ProxyConfig,
	registry *prometheus.Registry,
	tlsConfig *tls.Config,
	logger log.Logger,
) *Server {
	logger = logger.WithSubsystem("proxy")

	router := gin.New()
	s := &Server{
		proxy: NewReverseProxy(upstreams, proxyConfig.Timeout, logger),
		httpServer: &http.Server{
			Handler:           router,
			TLSConfig:         tlsConfig,
			ReadTimeout:       proxyConfig.HTTP.ReadTimeout,
			ReadHeaderTimeout: proxyConfig.HTTP.ReadHeaderTimeout,
			WriteTimeout:      proxyConfig.HTTP.WriteTimeout,
			IdleTimeout:       proxyConfig.HTTP.IdleTimeout,
			MaxHeaderBytes:    proxyConfig.HTTP.MaxHeaderBytes,
			ErrorLog:          logger.StdLogger(zapcore.WarnLevel),
		},
		logger: logger,
	}

	// Recover from panics.
	router.Use(gin.CustomRecoveryWithWriter(nil, s.panicRoute))

	router.Use(middleware.NewLogger(proxyConfig.AccessLog, logger))

	metrics := middleware.NewMetrics("proxy")
	if registry != nil {
		metrics.Register(registry)
	}
	router.Use(metrics.Handler())

	router.NoRoute(s.proxyRoute)

	return s
}

func (s *Server) Serve(ln net.Listener) error {
	s.logger.Info(
		"starting proxy server",
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

// proxyRoute handles proxied requests from proxy clients.
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
