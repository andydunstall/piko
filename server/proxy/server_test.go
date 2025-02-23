package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/andydunstall/piko/pkg/auth"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/websocket"
	"github.com/andydunstall/piko/server/config"
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

func echoListener(ln net.Listener) {
	conn, err := ln.Accept()
	if err != nil {
		panic("accept: " + err.Error())
	}

	buf := make([]byte, 512)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			panic("read: " + err.Error())
		}
		if _, err := conn.Write(buf[:n]); err != nil {
			panic("write: " + err.Error())
		}
	}
}

type fakeVerifier struct {
	handler func(token string) (*auth.Token, error)
}

func (v *fakeVerifier) Verify(token string) (*auth.Token, error) {
	return v.handler(token)
}

var _ auth.Verifier = &fakeVerifier{}

// TestServer_HTTP tests proxying HTTP traffic to upstreams.
func TestServer_HTTP(t *testing.T) {
	// Tests proxying a request to the upstream.
	t.Run("ok", func(t *testing.T) {
		// Add an upstream HTTP server.
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

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		s := NewServer(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return &tcpUpstream{
						addr: upstreamServer.Listener.Addr().String(),
					}, true
				},
			},
			config.Default().Proxy,
			nil,
			nil,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, s.Serve(ln))
		}()
		defer s.Shutdown(context.TODO())

		b := bytes.NewReader([]byte("foo"))
		url := fmt.Sprintf("http://%s/foo/bar?a=b", ln.Addr().String())
		req, _ := http.NewRequest(http.MethodGet, url, b)
		req.Header.Add("x-piko-endpoint", "my-endpoint")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		buf := new(strings.Builder)
		// nolint
		io.Copy(buf, resp.Body)
		assert.Equal(t, "bar", buf.String())
	})

	// Tests a request times out when upstream doesn't respond.
	t.Run("timeout", func(t *testing.T) {
		blockCh := make(chan struct{})
		upstreamServer := httptest.NewServer(http.HandlerFunc(
			func(_ http.ResponseWriter, _ *http.Request) {
				<-blockCh
			},
		))
		defer upstreamServer.Close()
		defer close(blockCh)

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		conf := config.Default().Proxy
		conf.Timeout = time.Millisecond
		s := NewServer(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return &tcpUpstream{
						addr: upstreamServer.Listener.Addr().String(),
					}, true
				},
			},
			conf,
			nil,
			nil,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, s.Serve(ln))
		}()
		defer s.Shutdown(context.TODO())

		url := fmt.Sprintf("http://%s/", ln.Addr().String())
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Add("x-piko-endpoint", "my-endpoint")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "upstream timeout", m.Error)
	})

	// Tests a request returns an error when the upstream is unreachable.
	t.Run("upstream unreachable", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		s := NewServer(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return &tcpUpstream{
						// Unreachable address.
						addr: "localhost:55555",
					}, true
				},
			},
			config.Default().Proxy,
			nil,
			nil,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, s.Serve(ln))
		}()
		defer s.Shutdown(context.TODO())

		url := fmt.Sprintf("http://%s/", ln.Addr().String())
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Add("x-piko-endpoint", "my-endpoint")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "upstream unreachable", m.Error)
	})

	// Tests the server returns an error if there are no upstreams for the
	// requested endpoint.
	t.Run("no available upstreams", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		s := NewServer(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return nil, false
				},
			},
			config.Default().Proxy,
			nil,
			nil,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, s.Serve(ln))
		}()
		defer s.Shutdown(context.TODO())

		url := fmt.Sprintf("http://%s/", ln.Addr().String())
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Add("x-piko-endpoint", "my-endpoint")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "no available upstreams", m.Error)
	})

	// Tests the server returns an error if the request is missing an endpoint
	// ID.
	t.Run("missing endpoint id", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		s := NewServer(
			nil,
			config.Default().Proxy,
			nil,
			nil,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, s.Serve(ln))
		}()
		defer s.Shutdown(context.TODO())

		url := fmt.Sprintf("http://%s/", ln.Addr().String())
		req, _ := http.NewRequest(http.MethodGet, url, nil)

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "missing endpoint id", m.Error)
	})
}

// TestServer_TCP tests proxying TCP traffic to upstreams.
func TestServer_TCP(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		echoLn, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		defer echoLn.Close()

		go echoListener(echoLn)

		server := NewServer(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return &tcpUpstream{
						addr: echoLn.Addr().String(),
					}, true
				},
			},
			config.Default().Proxy,
			nil,
			nil,
			nil,
			log.NewNopLogger(),
		)

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		defer ln.Close()

		// nolint
		go server.Serve(ln)

		conn, err := websocket.Dial(
			context.TODO(),
			"ws://"+ln.Addr().String()+"/_piko/v1/tcp/my-endpoint",
		)
		assert.NoError(t, err)

		// Test writing bytes to the upstream and waiting for them to be
		// echoed back.

		buf := make([]byte, 512)
		for i := 0; i != 10; i++ {
			_, err = conn.Write([]byte("foo"))
			assert.NoError(t, err)

			n, err := conn.Read(buf)
			assert.NoError(t, err)
			assert.Equal(t, 3, n)
		}
	})

	t.Run("upstream unreachable", func(t *testing.T) {
		server := NewServer(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return &tcpUpstream{
						// Unreachable address.
						addr: "localhost:55555",
					}, true
				},
			},
			config.Default().Proxy,
			nil,
			nil,
			nil,
			log.NewNopLogger(),
		)

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		defer ln.Close()

		// nolint
		go server.Serve(ln)

		_, err = websocket.Dial(
			context.TODO(),
			"ws://"+ln.Addr().String()+"/_piko/v1/tcp/my-endpoint",
		)
		assert.ErrorContains(t, err, "502: upstream unreachable")
	})
}

func TestServer_Authentication(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		// Add an upstream HTTP server.
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

		verifier := auth.NewMultiTenantVerifier(&fakeVerifier{
			handler: func(token string) (*auth.Token, error) {
				assert.Equal(t, "123", token)
				return &auth.Token{
					Expiry:    time.Now().Add(time.Hour),
					Endpoints: []string{"my-endpoint"},
				}, nil
			},
		}, nil)

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		s := NewServer(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return &tcpUpstream{
						addr: upstreamServer.Listener.Addr().String(),
					}, true
				},
			},
			config.Default().Proxy,
			nil,
			verifier,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, s.Serve(ln))
		}()
		defer s.Shutdown(context.TODO())

		b := bytes.NewReader([]byte("foo"))
		url := fmt.Sprintf("http://%s/foo/bar?a=b", ln.Addr().String())
		req, _ := http.NewRequest(http.MethodGet, url, b)
		req.Header.Add("x-piko-endpoint", "my-endpoint")
		req.Header.Add("Authorization", "Bearer 123")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		buf := new(strings.Builder)
		// nolint
		io.Copy(buf, resp.Body)
		assert.Equal(t, "bar", buf.String())
	})

	t.Run("endpoint not permitted", func(t *testing.T) {
		verifier := auth.NewMultiTenantVerifier(&fakeVerifier{
			handler: func(token string) (*auth.Token, error) {
				assert.Equal(t, "123", token)
				return &auth.Token{
					Expiry:    time.Now().Add(time.Hour),
					Endpoints: []string{"my-endpoint"},
				}, nil
			},
		}, nil)

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		s := NewServer(
			nil,
			config.Default().Proxy,
			nil,
			verifier,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, s.Serve(ln))
		}()
		defer s.Shutdown(context.TODO())

		url := fmt.Sprintf("http://%s/foo", ln.Addr().String())
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		// Add an unauthorized endpoint ID.
		req.Header.Add("x-piko-endpoint", "unauthorized-endpoint")
		req.Header.Add("Authorization", "Bearer 123")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	// Tests authenticating with a token that doesn't contain any endpoints
	// (meaning the client can access ALL endpoints).
	t.Run("token missing endpoints", func(t *testing.T) {
		// Add an upstream HTTP server.
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

		verifier := auth.NewMultiTenantVerifier(&fakeVerifier{
			handler: func(token string) (*auth.Token, error) {
				assert.Equal(t, "123", token)
				return &auth.Token{
					Expiry: time.Now().Add(time.Hour),
				}, nil
			},
		}, nil)

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		s := NewServer(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return &tcpUpstream{
						addr: upstreamServer.Listener.Addr().String(),
					}, true
				},
			},
			config.Default().Proxy,
			nil,
			verifier,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, s.Serve(ln))
		}()
		defer s.Shutdown(context.TODO())

		b := bytes.NewReader([]byte("foo"))
		url := fmt.Sprintf("http://%s/foo/bar?a=b", ln.Addr().String())
		req, _ := http.NewRequest(http.MethodGet, url, b)
		req.Header.Add("x-piko-endpoint", "my-endpoint")
		req.Header.Add("Authorization", "Bearer 123")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		buf := new(strings.Builder)
		// nolint
		io.Copy(buf, resp.Body)
		assert.Equal(t, "bar", buf.String())
	})

	t.Run("unauthenticated", func(t *testing.T) {
		verifier := auth.NewMultiTenantVerifier(&fakeVerifier{
			handler: func(token string) (*auth.Token, error) {
				assert.Equal(t, "123", token)
				return nil, auth.ErrInvalidToken
			},
		}, nil)

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		s := NewServer(
			nil,
			config.Default().Proxy,
			nil,
			verifier,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, s.Serve(ln))
		}()
		defer s.Shutdown(context.TODO())

		url := fmt.Sprintf("http://%s/foo/bar", ln.Addr().String())
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Add("x-piko-endpoint", "my-endpoint")

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
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
