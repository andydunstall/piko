package reverseproxy

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/andydunstall/piko/pkg/log"
	"go.uber.org/zap/zapcore"
)

// Handler implements a reverse proxy HTTP handler that accepts requests from
// downstream clients and forwards them to upstream services.
type Handler struct {
	upstreams UpstreamPool

	proxy *httputil.ReverseProxy

	logger log.Logger
}

func NewHandler(upstreams UpstreamPool, logger log.Logger) *Handler {
	logger = logger.WithSubsystem("reverseproxy")
	handler := &Handler{
		upstreams: upstreams,
		logger:    logger,
	}

	transport := &http.Transport{
		DialContext: handler.dialContext,
		// 'connections' to the upstream are multiplexed over a single TCP
		// connection so theres no overhead to creating new connections,
		// therefore it doesn't make sense to keep them alive.
		DisableKeepAlives: true,
	}
	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(&url.URL{
				Scheme:   "http",
				Host:     r.In.Host,
				Path:     r.In.URL.Path,
				RawQuery: r.In.URL.RawQuery,
			})
		},
		ErrorLog: logger.StdLogger(zapcore.WarnLevel),
	}
	handler.proxy = proxy
	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	endpointID := endpointIDFromRequest(r)
	if endpointID == "" {
		h.logger.Warn("request: missing endpoint id")
		http.Error(w, `{"message": "missing endpoint id"}`, http.StatusBadGateway)
		return
	}

	// nolint
	ctx := context.WithValue(r.Context(), "_piko_endpoint", endpointID)
	r = r.WithContext(ctx)

	h.proxy.ServeHTTP(w, r)
}

// dialContext dials the endpoint ID in ctx. This is a bit of a hack to work
// with http.Transport.
func (h *Handler) dialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	// TODO(andydunstall): Alternatively wrap Transport.RoundTrip and first
	// parse the endpoint ID, then decide whether to forward to a local
	// connection or a remote node.
	endpointID := ctx.Value("_piko_endpoint").(string)
	return h.upstreams.Dial(endpointID)
}

// endpointIDFromRequest returns the endpoint ID from the HTTP request, or an
// empty string if no endpoint ID is specified.
//
// This will check both the 'x-piko-endpoint' header and 'Host' header, where
// x-piko-endpoint takes precedence.
func endpointIDFromRequest(r *http.Request) string {
	endpointID := r.Header.Get("x-piko-endpoint")
	if endpointID != "" {
		return endpointID
	}

	host := r.Host
	if host != "" && strings.Contains(host, ".") {
		// If a host is given and contains a separator, use the bottom-level
		// domain as the endpoint ID.
		//
		// Such as if the domain is 'xyz.piko.example.com', then 'xyz' is the
		// endpoint ID.
		return strings.Split(host, ".")[0]
	}

	return ""
}
