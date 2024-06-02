//go:build integration

package endpoint

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
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

func TestEndpoint_Forward(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {},
		))
		defer upstream.Close()

		endpoint := NewEndpoint(config.EndpointConfig{
			ID:   "my-endpoint",
			Addr: upstream.URL,
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
