package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/middleware"
	"github.com/andydunstall/piko/server/config"
	"github.com/andydunstall/piko/server/upstream"
)

type Server struct {
	httpProxy *HTTPProxy
	tcpProxy  *TCPProxy

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

	httpProxy := NewHTTPProxy(upstreams, proxyConfig.Timeout, logger)

	router := gin.New()
	s := &Server{
		httpProxy: httpProxy,
		tcpProxy:  NewTCPProxy(upstreams, httpProxy, logger),
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

	s.registerRoutes(router)

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
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Server) registerRoutes(router *gin.Engine) {
	// All /_piko routes are reserved.
	piko := router.Group("/_piko")
	v1 := piko.Group("/v1")
	v1.GET("/tcp/:endpointID", s.proxyTCPRoute)

	router.NoRoute(s.proxyHTTPRoute)
}

func (s *Server) proxyHTTPRoute(c *gin.Context) {
	s.httpProxy.ServeHTTP(c.Writer, c.Request)
}

func (s *Server) proxyTCPRoute(c *gin.Context) {
	endpointID := c.Param("endpointID")
	s.tcpProxy.ServeHTTP(c.Writer, c.Request, endpointID)
}

func (s *Server) panicRoute(c *gin.Context, err any) {
	s.logger.Error(
		"handler panic",
		zap.String("path", c.FullPath()),
		zap.Any("err", err),
	)
	c.AbortWithStatus(http.StatusInternalServerError)
}

// EndpointIDFromRequest returns the endpoint ID from the HTTP request, or an
// empty string if no endpoint ID is specified.
//
// This will check both the 'x-piko-endpoint' header and 'Host' header, where
// x-piko-endpoint takes precedence.
func EndpointIDFromRequest(r *http.Request) string {
	endpointID := r.Header.Get("x-piko-endpoint")
	if endpointID != "" {
		return endpointID
	}

	// Strip the port if given.
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

	if host == "" {
		return ""
	}
	if net.ParseIP(host) != nil {
		// Ignore IP addresses.
		return ""
	}
	if strings.Contains(host, ".") {
		// If a host is given and contains a separator, use the bottom-level
		// domain as the endpoint ID.
		//
		// Such as if the domain is 'xyz.piko.example.com', then 'xyz' is the
		// endpoint ID.
		return strings.Split(host, ".")[0]
	}

	return ""
}

func init() {
	// Disable Gin debug logs.
	gin.SetMode(gin.ReleaseMode)
}
