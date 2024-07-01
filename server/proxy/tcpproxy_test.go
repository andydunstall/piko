package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/websocket"
	"github.com/andydunstall/piko/server/config"
	"github.com/andydunstall/piko/server/upstream"
)

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

func TestTCPProxy_Forward(t *testing.T) {
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
			config.ProxyConfig{},
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
		proxy := NewTCPProxy(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return &tcpUpstream{
						addr: "localhost:55555",
					}, true
				},
			},
			nil,
			log.NewNopLogger(),
		)

		r := httptest.NewRequest(http.MethodGet, "/", nil)

		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r, "my-endpoint")

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "upstream unreachable", m.Error)
	})

	t.Run("no available upstreams", func(t *testing.T) {
		proxy := NewTCPProxy(
			&fakeManager{
				handler: func(endpointID string, allowForward bool) (upstream.Upstream, bool) {
					assert.Equal(t, "my-endpoint", endpointID)
					assert.True(t, allowForward)
					return nil, false
				},
			},
			nil,
			log.NewNopLogger(),
		)

		r := httptest.NewRequest(http.MethodGet, "/", nil)

		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r, "my-endpoint")

		resp := w.Result()
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "no available upstreams", m.Error)
	})
}
