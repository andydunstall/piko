package reverseproxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dragonflydb/piko/agent/config"
	"github.com/dragonflydb/piko/pkg/log"
)

func TestReverseProxy_Forward(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(
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
		defer upstream.Close()

		proxy := NewReverseProxy(config.ListenerConfig{
			EndpointID: "my-endpoint",
			Addr:       upstream.URL,
		}, log.NewNopLogger())

		b := bytes.NewReader([]byte("foo"))
		r := httptest.NewRequest(http.MethodGet, "/foo/bar?a=b", b)

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
		upstream := httptest.NewServer(http.HandlerFunc(
			func(_ http.ResponseWriter, _ *http.Request) {
				<-blockCh
			},
		))
		defer upstream.Close()
		defer close(blockCh)

		proxy := NewReverseProxy(config.ListenerConfig{
			EndpointID: "my-endpoint",
			Addr:       upstream.URL,
			Timeout:    time.Millisecond * 1,
		}, log.NewNopLogger())

		r := httptest.NewRequest(http.MethodGet, "/", nil)

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
		proxy := NewReverseProxy(config.ListenerConfig{
			EndpointID: "my-endpoint",
			Addr:       "localhost:55555",
		}, log.NewNopLogger())

		r := httptest.NewRequest(http.MethodGet, "/", nil)

		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "upstream unreachable", m.Error)
	})
}
