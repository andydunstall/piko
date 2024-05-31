//go:build integration

package endpoint

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/andydunstall/piko/agentv2/config"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeListener struct {
	net.Listener

	endpointID string
}

func (l *fakeListener) EndpointID() string {
	return l.endpointID
}

type upstreamServer struct {
	ln     net.Listener
	server *http.Server
}

func newUpstreamServer(handler func(http.ResponseWriter, *http.Request)) (*upstreamServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	return &upstreamServer{
		server: &http.Server{
			Addr:    ln.Addr().String(),
			Handler: mux,
		},
		ln: ln,
	}, nil
}

func (s *upstreamServer) Addr() string {
	return s.ln.Addr().String()
}

func (s *upstreamServer) Serve() error {
	return s.server.Serve(s.ln)
}

func (s *upstreamServer) Close() error {
	return s.server.Close()
}

func TestEndpoint_Forward(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		upstream, err := newUpstreamServer(func(w http.ResponseWriter, r *http.Request) {
		})
		require.NoError(t, err)
		go func() {
			if err := upstream.Serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				require.NoError(t, err)
			}
		}()
		defer upstream.Close()

		endpoint := NewEndpoint(config.EndpointConfig{
			ID:   "my-endpoint",
			Addr: upstream.Addr(),
		}, log.NewNopLogger())

		tcpLn, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		pikoLn := &fakeListener{
			Listener:   tcpLn,
			endpointID: "my-endpoint",
		}

		go func() {
			if err := endpoint.Serve(pikoLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
				require.NoError(t, err)
			}
		}()
		defer endpoint.Shutdown(context.TODO())

		url := fmt.Sprintf("http://%s/foo/bar", pikoLn.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("no upstream", func(t *testing.T) {
		endpoint := NewEndpoint(config.EndpointConfig{
			ID:   "my-endpoint",
			Addr: "55555",
		}, log.NewNopLogger())

		tcpLn, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		pikoLn := &fakeListener{
			Listener:   tcpLn,
			endpointID: "my-endpoint",
		}

		go func() {
			if err := endpoint.Serve(pikoLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
				require.NoError(t, err)
			}
		}()
		defer endpoint.Shutdown(context.TODO())

		url := fmt.Sprintf("http://%s/foo/bar", pikoLn.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	})
}
