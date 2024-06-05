package endpoint

import (
	"net/http"
	"net/http/httputil"

	"github.com/andydunstall/piko/agentv2/config"
	"github.com/andydunstall/piko/pkg/log"
	"go.uber.org/zap/zapcore"
)

// Handler implements a reverse proxy HTTP handler that accepts requests from
// downstream clients and forwards them to upstream services.
type Handler struct {
	proxy *httputil.ReverseProxy

	logger log.Logger
}

func NewHandler(conf config.EndpointConfig, logger log.Logger) *Handler {
	logger = logger.WithSubsystem("endpoint.reverseproxy")

	u, ok := conf.URL()
	if !ok {
		// We've already verified the address on boot so don't need to handle
		// the error.
		panic("invalid endpoint addr: " + conf.Addr)
	}

	return &Handler{
		proxy: &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.SetURL(u)
			},
			ErrorLog: logger.StdLogger(zapcore.WarnLevel),
		},
		logger: logger,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.proxy.ServeHTTP(w, r)
}
