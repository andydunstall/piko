package upstream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/agent/reverseproxy"
	"github.com/andydunstall/piko/client"
	"github.com/andydunstall/piko/pkg/log"
)

type Upstream struct {
	endpointID string
	serverURL  string
	logger     log.Logger
}

func NewUpstream(endpointID string, serverURL string, logger log.Logger) *Upstream {
	return &Upstream{
		endpointID: endpointID,
		serverURL:  serverURL,
		logger:     logger,
	}
}

func (u *Upstream) Run(ctx context.Context) error {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Note can't use io.Copy as not supported by http.ResponseWriter.
		b, err := io.ReadAll(r.Body)
		if err != nil {
			panic(fmt.Sprintf("read body: %s", err.Error()))
		}
		n, err := w.Write(b)
		if err != nil {
			panic(fmt.Sprintf("write bytes: %d: %s", n, err))
		}
	}))
	defer server.Close()

	url, _ := url.Parse(u.serverURL)
	upstream := client.Upstream{
		URL:    url,
		Logger: u.logger,
	}
	ln, err := upstream.Listen(ctx, u.endpointID)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	proxy := reverseproxy.NewServer(config.ListenerConfig{
		EndpointID: u.endpointID,
		Addr:       server.Listener.Addr().String(),
	}, nil, u.logger)
	go func() {
		_ = proxy.Serve(ln)
	}()

	<-ctx.Done()
	proxy.Shutdown(context.Background())
	return nil
}
