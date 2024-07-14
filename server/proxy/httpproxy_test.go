package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/upstream"
)

type fakeManager struct {
	handler func(endpointID string, allowForward bool) (upstream.Upstream, bool)
}

func (m *fakeManager) Select(
	endpointID string,
	allowForward bool,
) (upstream.Upstream, bool) {
	return m.handler(endpointID, allowForward)
}

func (m *fakeManager) AddConn(_ upstream.Upstream) {
}

func (m *fakeManager) RemoveConn(_ upstream.Upstream) {
}

type tcpUpstream struct {
	addr    string
	forward bool
}

func (u *tcpUpstream) Dial() (net.Conn, error) {
	return net.Dial("tcp", u.addr)
}

func (u *tcpUpstream) EndpointID() string {
	return "my-endpoint"
}

func (u *tcpUpstream) Forward() bool {
	return u.forward
}

func TestHTTPProxy_Forward(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/foo/bar", r.URL.Path)
				assert.Equal(t, "a=b", r.URL.RawQuery)

				buf := new(strings.Builder)
				// nolint
				io.Copy(buf, r.Body)
				assert.Equal(t, "foo", buf.String())

				// nolint
				w.Write([]byte("bar"))
			},
		))
		defer server.Close()

		proxy := NewHTTPProxy(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return &tcpUpstream{
						addr: server.Listener.Addr().String(),
					}, true
				},
			},
			time.Second,
			log.NewNopLogger(),
		)

		b := bytes.NewReader([]byte("foo"))
		r := httptest.NewRequest(http.MethodGet, "/foo/bar?a=b", b)
		r.Header.Add("x-piko-endpoint", "my-endpoint")

		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		buf := new(strings.Builder)
		// nolint
		io.Copy(buf, resp.Body)
		assert.Equal(t, "bar", buf.String())
	})

	t.Run("timeout", func(t *testing.T) {
		blockCh := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(
			func(_ http.ResponseWriter, _ *http.Request) {
				<-blockCh
			},
		))
		defer server.Close()
		defer close(blockCh)

		proxy := NewHTTPProxy(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return &tcpUpstream{
						addr: server.Listener.Addr().String(),
					}, true
				},
			},
			time.Millisecond,
			log.NewNopLogger(),
		)

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Add("x-piko-endpoint", "my-endpoint")

		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "upstream timeout", m.Error)
	})

	t.Run("upstream unreachable", func(t *testing.T) {
		proxy := NewHTTPProxy(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return &tcpUpstream{
						addr: "localhost:55555",
					}, true
				},
			},
			time.Second,
			log.NewNopLogger(),
		)

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Add("x-piko-endpoint", "my-endpoint")

		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "upstream unreachable", m.Error)
	})

	t.Run("no available upstreams", func(t *testing.T) {
		proxy := NewHTTPProxy(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return nil, false
				},
			},
			time.Second,
			log.NewNopLogger(),
		)

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Add("x-piko-endpoint", "my-endpoint")

		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "no available upstreams", m.Error)
	})

	t.Run("no available upstreams forwarded", func(t *testing.T) {
		proxy := NewHTTPProxy(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.False(t, allowForward)
					return nil, false
				},
			},
			time.Second,
			log.NewNopLogger(),
		)

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Add("x-piko-endpoint", "my-endpoint")
		r.Header.Add("x-piko-forward", "true")

		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "no available upstreams", m.Error)
	})

	t.Run("missing endpoint id", func(t *testing.T) {
		proxy := NewHTTPProxy(nil, time.Second, log.NewNopLogger())

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		// The host must have a '.' separator to be parsed as an endpoint ID.
		r.Host = "foo"

		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "missing endpoint id", m.Error)
	})
}

func TestEndpointIDFromRequest(t *testing.T) {
	t.Run("host header", func(t *testing.T) {
		endpointID := EndpointIDFromRequest(&http.Request{
			Host: "my-endpoint.piko.com:9000",
		})
		assert.Equal(t, "my-endpoint", endpointID)
	})

	t.Run("x-piko-endpoint header", func(t *testing.T) {
		header := make(http.Header)
		header.Add("x-piko-endpoint", "my-endpoint")
		endpointID := EndpointIDFromRequest(&http.Request{
			// Even though the host header is provided, 'x-piko-endpoint'
			// takes precedence.
			Host:   "another-endpoint.piko.com:9000",
			Header: header,
		})
		assert.Equal(t, "my-endpoint", endpointID)
	})

	t.Run("ip address", func(t *testing.T) {
		endpointID := EndpointIDFromRequest(&http.Request{
			Host: "127.0.0.1:9000",
		})
		assert.Equal(t, "", endpointID)
	})

	t.Run("no separator", func(t *testing.T) {
		endpointID := EndpointIDFromRequest(&http.Request{
			Host: "localhost:9000",
		})
		assert.Equal(t, "", endpointID)
	})

	t.Run("empty host", func(t *testing.T) {
		endpointID := EndpointIDFromRequest(&http.Request{
			Host: "",
		})
		assert.Equal(t, "", endpointID)
	})
}
