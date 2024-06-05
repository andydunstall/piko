package endpoint

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/andydunstall/piko/agentv2/config"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Endpoint handles connections for the endpoint and forwards traffic the
// upstream listener.
type Endpoint struct {
	id string

	handler *Handler

	router *gin.Engine

	httpServer *http.Server

	logger log.Logger
}

func NewEndpoint(conf config.EndpointConfig, logger log.Logger) *Endpoint {
	logger = logger.WithSubsystem("endpoint").With(
		zap.String("endpoint-id", conf.ID),
	)

	router := gin.New()
	endpoint := &Endpoint{
		id:      conf.ID,
		handler: NewHandler(conf, logger),
		router:  router,
		httpServer: &http.Server{
			Handler:  router,
			ErrorLog: logger.StdLogger(zapcore.WarnLevel),
		},
		logger: logger,
	}

	// Recover from panics.
	endpoint.router.Use(gin.CustomRecoveryWithWriter(nil, endpoint.panicRoute))

	endpoint.router.Use(NewLoggerMiddleware(conf.AccessLog, logger))

	endpoint.registerRoutes()

	return endpoint
}

func (e *Endpoint) Serve(ln net.Listener) error {
	e.logger.Info("starting endpoint")
	if err := e.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http serve: %w", err)
	}
	return nil
}

func (e *Endpoint) Shutdown(ctx context.Context) error {
	return e.httpServer.Shutdown(ctx)
}

func (e *Endpoint) registerRoutes() {
	// Handle not found routes, which includes all proxied endpoints.
	e.router.NoRoute(e.notFoundRoute)
}

// proxyRoute handles proxied requests from proxy clients.
func (e *Endpoint) proxyRoute(c *gin.Context) {
	e.handler.ServeHTTP(c.Writer, c.Request)
}

func (e *Endpoint) notFoundRoute(c *gin.Context) {
	e.proxyRoute(c)
}

func (e *Endpoint) panicRoute(c *gin.Context, err any) {
	e.logger.Error(
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
