//go:build integration

package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeProxy struct {
	handler func(ctx context.Context, r *http.Request) *http.Response
}

func (p *fakeProxy) Request(ctx context.Context, r *http.Request) *http.Response {
	return p.handler(ctx, r)
}

func TestServer_ProxyRequest(t *testing.T) {
	t.Run("forwarded", func(t *testing.T) {
		handler := func(ctx context.Context, r *http.Request) *http.Response {
			assert.Equal(t, "/foo/bar", r.URL.Path)

			header := make(http.Header)
			header.Add("h1", "v1")
			header.Add("h2", "v2")
			header.Add("h3", "v3")
			body := bytes.NewReader([]byte("foo"))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(body),
			}
		}

		proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		proxyServer := NewServer(
			proxyLn,
			&fakeProxy{handler: handler},
			&config.ProxyConfig{},
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, proxyServer.Serve())
		}()
		defer proxyServer.Shutdown(context.TODO())

		url := fmt.Sprintf("http://%s/foo/bar", proxyLn.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "v1", resp.Header.Get("h1"))
		assert.Equal(t, "v2", resp.Header.Get("h2"))
		assert.Equal(t, "v3", resp.Header.Get("h3"))

		buf := new(bytes.Buffer)
		//nolint
		buf.ReadFrom(resp.Body)
		assert.Equal(t, []byte("foo"), buf.Bytes())
	})
}

func TestServer_HandlePanic(t *testing.T) {
	handler := func(ctx context.Context, r *http.Request) *http.Response {
		panic("fail")
	}

	proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	proxyServer := NewServer(
		proxyLn,
		&fakeProxy{handler: handler},
		&config.ProxyConfig{},
		nil,
		log.NewNopLogger(),
	)
	go func() {
		require.NoError(t, proxyServer.Serve())
	}()
	defer proxyServer.Shutdown(context.TODO())

	url := fmt.Sprintf("http://%s/foo/bar", proxyLn.Addr().String())
	resp, err := http.Get(url)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
