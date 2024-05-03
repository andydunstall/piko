package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/netmap"
	"github.com/stretchr/testify/assert"
)

type fakeConn struct {
	endpointID string
	addr       string
	handler    func(r *http.Request) (*http.Response, error)
}

func (c *fakeConn) EndpointID() string {
	return c.endpointID
}

func (c *fakeConn) Addr() string {
	return c.addr
}

func (c *fakeConn) Request(
	_ context.Context,
	r *http.Request,
) (*http.Response, error) {
	return c.handler(r)
}

type fakeForwarder struct {
	handler func(addr string, r *http.Request) (*http.Response, error)
}

func (f *fakeForwarder) Request(
	_ context.Context,
	addr string,
	r *http.Request,
) (*http.Response, error) {
	return f.handler(addr, r)
}

func TestProxy(t *testing.T) {
	t.Run("forward request remote ok", func(t *testing.T) {
		networkMap := netmap.NewNetworkMap(&netmap.Node{}, log.NewNopLogger())
		networkMap.AddNode(&netmap.Node{
			ID:        "node-1",
			ProxyAddr: "1.2.3.4:1234",
			Endpoints: map[string]int{
				"my-endpoint": 5,
			},
		})

		handler := func(addr string, r *http.Request) (*http.Response, error) {
			assert.Equal(t, "1.2.3.4:1234", addr)
			assert.Equal(t, "true", r.Header.Get("x-pico-forward"))
			return &http.Response{
				StatusCode: http.StatusOK,
			}, nil
		}
		forwarder := &fakeForwarder{
			handler: handler,
		}
		proxy := NewProxy(networkMap, WithForwarder(forwarder))

		header := make(http.Header)
		resp := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host:   "my-endpoint.pico.com",
			Header: header,
		})

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("forward request remote endpoint timeout", func(t *testing.T) {
		networkMap := netmap.NewNetworkMap(&netmap.Node{}, log.NewNopLogger())
		networkMap.AddNode(&netmap.Node{
			ID:        "node-1",
			ProxyAddr: "1.2.3.4:1234",
			Endpoints: map[string]int{
				"my-endpoint": 5,
			},
		})

		handler := func(addr string, r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("error: %w", context.DeadlineExceeded)
		}
		forwarder := &fakeForwarder{
			handler: handler,
		}
		proxy := NewProxy(networkMap, WithForwarder(forwarder))

		header := make(http.Header)
		resp := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host:   "my-endpoint.pico.com",
			Header: header,
		})

		assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)

		var m errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "endpoint timeout", m.Error)
	})

	t.Run("forward request remote endpoint unreachable", func(t *testing.T) {
		networkMap := netmap.NewNetworkMap(&netmap.Node{}, log.NewNopLogger())
		networkMap.AddNode(&netmap.Node{
			ID:        "node-1",
			ProxyAddr: "1.2.3.4:1234",
			Endpoints: map[string]int{
				"my-endpoint": 5,
			},
		})

		handler := func(addr string, r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("unknown error")
		}
		forwarder := &fakeForwarder{
			handler: handler,
		}
		proxy := NewProxy(networkMap, WithForwarder(forwarder))

		header := make(http.Header)
		resp := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host:   "my-endpoint.pico.com",
			Header: header,
		})
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		var m errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "endpoint unreachable", m.Error)
	})

	t.Run("forward request remote endpoint not found", func(t *testing.T) {
		proxy := NewProxy(
			netmap.NewNetworkMap(&netmap.Node{}, log.NewNopLogger()),
		)

		header := make(http.Header)
		resp := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host:   "my-endpoint.pico.com",
			Header: header,
		})

		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		var m errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "endpoint not found", m.Error)
	})

	t.Run("forward local ok", func(t *testing.T) {
		handler := func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
			}, nil
		}
		conn := &fakeConn{
			endpointID: "my-endpoint",
			addr:       "1.1.1.1",
			handler:    handler,
		}

		proxy := NewProxy(
			netmap.NewNetworkMap(&netmap.Node{}, log.NewNopLogger()),
		)

		proxy.AddConn(conn)

		resp := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host: "my-endpoint.pico.com",
		})
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("forward request local endpoint timeout", func(t *testing.T) {
		handler := func(r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("error: %w", context.DeadlineExceeded)
		}
		conn := &fakeConn{
			endpointID: "my-endpoint",
			addr:       "1.1.1.1",
			handler:    handler,
		}

		proxy := NewProxy(
			netmap.NewNetworkMap(&netmap.Node{}, log.NewNopLogger()),
		)

		proxy.AddConn(conn)

		resp := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host: "my-endpoint.pico.com",
		})
		assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)

		var m errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "endpoint timeout", m.Error)
	})

	t.Run("forward request local endpoint unreachable", func(t *testing.T) {
		handler := func(r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("unknown error")
		}
		conn := &fakeConn{
			endpointID: "my-endpoint",
			addr:       "1.1.1.1",
			handler:    handler,
		}

		proxy := NewProxy(
			netmap.NewNetworkMap(&netmap.Node{}, log.NewNopLogger()),
		)

		proxy.AddConn(conn)

		resp := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host: "my-endpoint.pico.com",
		})
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		var m errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "endpoint unreachable", m.Error)
	})

	t.Run("forward request local endpoint not found", func(t *testing.T) {
		proxy := NewProxy(
			netmap.NewNetworkMap(&netmap.Node{}, log.NewNopLogger()),
		)

		header := make(http.Header)
		// Set forward header to avoid being forwarded to a remote node.
		header.Set("x-pico-forward", "true")
		req := &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host:   "my-endpoint.pico.com",
			Header: header,
		}

		resp := proxy.Request(context.TODO(), req)
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		var m errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "endpoint not found", m.Error)

		conn := &fakeConn{
			endpointID: "my-endpoint",
		}
		proxy.AddConn(conn)
		proxy.RemoveConn(conn)

		resp = proxy.Request(context.TODO(), req)
		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "endpoint not found", m.Error)
	})

	t.Run("add conn", func(t *testing.T) {
		networkMap := netmap.NewNetworkMap(
			&netmap.Node{
				ID: "local",
			}, log.NewNopLogger(),
		)
		proxy := NewProxy(networkMap)

		conn := &fakeConn{
			endpointID: "my-endpoint",
		}
		proxy.AddConn(conn)
		// Verify the netmap was updated.
		assert.Equal(t, map[string]int{
			"my-endpoint": 1,
		}, networkMap.LocalNode().Endpoints)

		proxy.RemoveConn(conn)
		assert.Equal(t, 0, len(networkMap.LocalNode().Endpoints))
	})

	t.Run("missing endpoint", func(t *testing.T) {
		proxy := NewProxy(
			netmap.NewNetworkMap(&netmap.Node{}, log.NewNopLogger()),
		)

		resp := proxy.Request(context.TODO(), &http.Request{
			URL: &url.URL{
				Path: "/foo",
			},
			Host: "localhost:9000",
		})

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var m errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "missing pico endpoint id", m.Error)
	})
}

func TestEndpointIDFromRequest(t *testing.T) {
	t.Run("host header", func(t *testing.T) {
		endpointID := endpointIDFromRequest(&http.Request{
			Host: "my-endpoint.pico.com:9000",
		})
		assert.Equal(t, "my-endpoint", endpointID)
	})

	t.Run("x-pico-endpoint header", func(t *testing.T) {
		header := make(http.Header)
		header.Add("x-pico-endpoint", "my-endpoint")
		endpointID := endpointIDFromRequest(&http.Request{
			// Even though the host header is provided, 'x-pico-endpoint'
			// takes precedence.
			Host:   "another-endpoint.pico.com:9000",
			Header: header,
		})
		assert.Equal(t, "my-endpoint", endpointID)
	})

	t.Run("no endpoint", func(t *testing.T) {
		endpointID := endpointIDFromRequest(&http.Request{
			Host: "localhost:9000",
		})
		assert.Equal(t, "", endpointID)
	})
}
