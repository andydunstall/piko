package reverseproxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/serverv2/upstream"
	"github.com/stretchr/testify/assert"
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

func TestReverseProxy(t *testing.T) {
	// Tests forwarding a request to the upstream with a path, query and body,
	// then checking the response is forwarded correctly.
	t.Run("ok", func(t *testing.T) {
		upstreamServer := httptest.NewServer(http.HandlerFunc(
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
		defer upstreamServer.Close()

		upstreamClient := upstream.NewTCPUpstream(
			"my-endpoint", upstreamServer.Listener.Addr().String(),
		)

		proxy := NewReverseProxy(&fakeManager{
			handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
				assert.Equal(t, "my-endpoint", endpointID)
				assert.True(t, allowForward)
				return upstreamClient, true
			},
		}, log.NewNopLogger())

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

	t.Run("no available upstreams", func(t *testing.T) {
		proxy := NewReverseProxy(&fakeManager{
			handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
				assert.Equal(t, "my-endpoint", endpointID)
				assert.True(t, allowForward)
				return nil, false
			},
		}, log.NewNopLogger())

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

	t.Run("missing endpoint id", func(t *testing.T) {
		proxy := NewReverseProxy(nil, log.NewNopLogger())

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

	t.Run("no endpoint", func(t *testing.T) {
		endpointID := EndpointIDFromRequest(&http.Request{
			Host: "localhost:9000",
		})
		assert.Equal(t, "", endpointID)
	})
}
