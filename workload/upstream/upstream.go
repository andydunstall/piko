package upstream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/andydunstall/piko/agent/client"
	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/agent/reverseproxy"
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
		//nolint
		io.Copy(w, r.Body)
	}))
	defer server.Close()

	client := client.New(client.WithURL(u.serverURL))

	ln, err := client.Listen(context.Background(), u.endpointID)
	if err != nil {
		return fmt.Errorf("listen: %s: %w", ln.EndpointID(), err)
	}
	defer ln.Close()

	proxy := reverseproxy.NewServer(config.ListenerConfig{
		EndpointID: u.endpointID,
		Addr:       server.Listener.Addr().String(),
	}, nil, log.NewNopLogger())
	go func() {
		_ = proxy.Serve(ln)
	}()

	<-ctx.Done()
	proxy.Shutdown(context.Background())
	return nil
}
