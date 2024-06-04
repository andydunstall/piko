package endpoint

import (
	"context"
	"net/http"
	"net/http/httputil"

	piko "github.com/andydunstall/piko/agentv2/client"
	"github.com/andydunstall/piko/agentv2/config"
	"github.com/andydunstall/piko/pkg/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Endpoint handles connections for the endpoint and forwards traffic the
// upstream listener.
type Endpoint struct {
	id string

	server *http.Server

	logger log.Logger
}

func NewEndpoint(conf config.EndpointConfig, logger log.Logger) *Endpoint {
	u, ok := conf.URL()
	if !ok {
		// We've already verified the address on boot so don't need to handle
		// the error.
		panic("invalid endpoint addr: " + conf.Addr)
	}

	// TODO(andydunstall): Configure timeouts, access log, ...
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
		},
		ErrorLog: logger.StdLogger(zapcore.WarnLevel),
	}
	return &Endpoint{
		id: conf.ID,
		server: &http.Server{
			Handler:  proxy,
			ErrorLog: logger.StdLogger(zapcore.WarnLevel),
		},
		logger: logger.WithSubsystem("endpoint").With(zap.String("endpoint-id", conf.ID)),
	}
}

// Serve serves connections on the listener.
func (e *Endpoint) Serve(ln piko.Listener) error {
	e.logger.Info("serving endpoint")
	return e.server.Serve(ln)
}

func (e *Endpoint) Shutdown(ctx context.Context) error {
	return e.server.Shutdown(ctx)
}
