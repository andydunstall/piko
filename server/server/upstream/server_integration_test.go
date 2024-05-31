//go:build integration

package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/andydunstall/piko/pkg/conn/websocket"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/rpc"
	"github.com/andydunstall/piko/pkg/testutil"
	"github.com/andydunstall/piko/server/auth"
	proxy "github.com/andydunstall/piko/server/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeProxy struct {
	addUpstreamCh    chan string
	removeUpstreamCh chan string
}

func newFakeProxy() *fakeProxy {
	return &fakeProxy{
		addUpstreamCh:    make(chan string),
		removeUpstreamCh: make(chan string),
	}
}

func (p *fakeProxy) AddConn(conn proxy.Conn) {
	p.addUpstreamCh <- conn.EndpointID()
}

func (p *fakeProxy) RemoveConn(conn proxy.Conn) {
	p.removeUpstreamCh <- conn.EndpointID()
}

type fakeVerifier struct {
	handler func(token string) (auth.EndpointToken, error)
}

func (v *fakeVerifier) VerifyEndpointToken(token string) (auth.EndpointToken, error) {
	return v.handler(token)
}

var _ auth.Verifier = &fakeVerifier{}

func TestServer_AddConn(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		upstreamLn, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		proxy := newFakeProxy()
		upstreamServer := NewServer(
			upstreamLn,
			proxy,
			nil,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, upstreamServer.Serve())
		}()
		defer upstreamServer.Shutdown(context.TODO())

		url := fmt.Sprintf(
			"ws://%s/piko/v1/listener/my-endpoint",
			upstreamLn.Addr().String(),
		)
		rpcServer := newRPCServer()
		conn, err := websocket.Dial(context.TODO(), url)
		require.NoError(t, err)

		// Add client stream and ensure upstream added to proxy.
		stream := rpc.NewStream(conn, rpcServer.Handler(), log.NewNopLogger())
		assert.Equal(t, "my-endpoint", <-proxy.addUpstreamCh)

		// Close client stream and ensure upstream removed from proxy.
		stream.Close()
		assert.Equal(t, "my-endpoint", <-proxy.removeUpstreamCh)
	})

	t.Run("authenticated", func(t *testing.T) {
		upstreamLn, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		verifier := &fakeVerifier{
			handler: func(token string) (auth.EndpointToken, error) {
				assert.Equal(t, "123", token)
				return auth.EndpointToken{
					Expiry:    time.Now().Add(time.Hour),
					Endpoints: []string{"my-endpoint"},
				}, nil
			},
		}

		proxy := newFakeProxy()
		upstreamServer := NewServer(
			upstreamLn,
			proxy,
			verifier,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, upstreamServer.Serve())
		}()
		defer upstreamServer.Shutdown(context.TODO())

		url := fmt.Sprintf(
			"ws://%s/piko/v1/listener/my-endpoint",
			upstreamLn.Addr().String(),
		)
		rpcServer := newRPCServer()
		conn, err := websocket.Dial(context.TODO(), url, websocket.WithToken("123"))
		require.NoError(t, err)

		// Add client stream and ensure upstream added to proxy.
		stream := rpc.NewStream(conn, rpcServer.Handler(), log.NewNopLogger())
		assert.Equal(t, "my-endpoint", <-proxy.addUpstreamCh)

		// Close client stream and ensure upstream removed from proxy.
		stream.Close()
		assert.Equal(t, "my-endpoint", <-proxy.removeUpstreamCh)
	})

	t.Run("token expires", func(t *testing.T) {
		upstreamLn, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		verifier := &fakeVerifier{
			handler: func(token string) (auth.EndpointToken, error) {
				assert.Equal(t, "123", token)
				return auth.EndpointToken{
					// Set a short expiry as we wait for the token to expire.
					Expiry:    time.Now().Add(time.Millisecond * 10),
					Endpoints: []string{"my-endpoint"},
				}, nil
			},
		}

		proxy := newFakeProxy()
		upstreamServer := NewServer(
			upstreamLn,
			proxy,
			verifier,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, upstreamServer.Serve())
		}()
		defer upstreamServer.Shutdown(context.TODO())

		url := fmt.Sprintf(
			"ws://%s/piko/v1/listener/my-endpoint",
			upstreamLn.Addr().String(),
		)
		rpcServer := newRPCServer()
		conn, err := websocket.Dial(context.TODO(), url, websocket.WithToken("123"))
		require.NoError(t, err)

		// Add client stream and ensure upstream added to proxy.
		stream := rpc.NewStream(conn, rpcServer.Handler(), log.NewNopLogger())
		defer stream.Close()
		assert.Equal(t, "my-endpoint", <-proxy.addUpstreamCh)

		// Wait for the token to expire and the server should close the
		// connection and remove it from the proxy.
		assert.Equal(t, "my-endpoint", <-proxy.removeUpstreamCh)
	})

	t.Run("endpoint not permitted", func(t *testing.T) {
		upstreamLn, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		verifier := &fakeVerifier{
			handler: func(token string) (auth.EndpointToken, error) {
				assert.Equal(t, "123", token)
				return auth.EndpointToken{
					Expiry:    time.Now().Add(time.Hour),
					Endpoints: []string{"foo"},
				}, nil
			},
		}

		upstreamServer := NewServer(
			upstreamLn,
			nil,
			verifier,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, upstreamServer.Serve())
		}()
		defer upstreamServer.Shutdown(context.TODO())

		url := fmt.Sprintf(
			"ws://%s/piko/v1/listener/my-endpoint",
			upstreamLn.Addr().String(),
		)
		_, err = websocket.Dial(context.TODO(), url, websocket.WithToken("123"))
		require.Error(t, err)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		upstreamLn, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		verifier := &fakeVerifier{
			handler: func(token string) (auth.EndpointToken, error) {
				assert.Equal(t, "123", token)
				return auth.EndpointToken{}, auth.ErrInvalidToken
			},
		}

		upstreamServer := NewServer(
			upstreamLn,
			nil,
			verifier,
			nil,
			log.NewNopLogger(),
		)
		go func() {
			require.NoError(t, upstreamServer.Serve())
		}()
		defer upstreamServer.Shutdown(context.TODO())

		url := fmt.Sprintf(
			"ws://%s/piko/v1/listener/my-endpoint",
			upstreamLn.Addr().String(),
		)
		_, err = websocket.Dial(context.TODO(), url, websocket.WithToken("123"))
		require.Error(t, err)
	})
}

func TestServer_TLS(t *testing.T) {
	rootCAPool, cert, err := testutil.LocalTLSServerCert()
	require.NoError(t, err)

	upstreamLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	tlsConfig := &tls.Config{}
	tlsConfig.Certificates = []tls.Certificate{cert}

	proxy := newFakeProxy()
	upstreamServer := NewServer(
		upstreamLn,
		proxy,
		nil,
		tlsConfig,
		log.NewNopLogger(),
	)
	go func() {
		require.NoError(t, upstreamServer.Serve())
	}()
	defer upstreamServer.Shutdown(context.TODO())

	t.Run("wss ok", func(t *testing.T) {
		url := fmt.Sprintf(
			"wss://%s/piko/v1/listener/my-endpoint",
			upstreamLn.Addr().String(),
		)
		clientTLSConfig := &tls.Config{
			RootCAs: rootCAPool,
		}
		conn, err := websocket.Dial(
			context.TODO(), url, websocket.WithTLSConfig(clientTLSConfig),
		)
		require.NoError(t, err)

		// Add client stream and ensure upstream added to proxy.
		rpcServer := newRPCServer()
		stream := rpc.NewStream(conn, rpcServer.Handler(), log.NewNopLogger())
		assert.Equal(t, "my-endpoint", <-proxy.addUpstreamCh)

		// Close client stream and ensure upstream removed from proxy.
		stream.Close()
		assert.Equal(t, "my-endpoint", <-proxy.removeUpstreamCh)
	})

	t.Run("wss bad ca", func(t *testing.T) {
		url := fmt.Sprintf(
			"wss://%s/piko/v1/listener/my-endpoint",
			upstreamLn.Addr().String(),
		)
		_, err := websocket.Dial(context.TODO(), url)
		require.ErrorContains(t, err, "certificate signed by unknown authority")
	})

	t.Run("ws", func(t *testing.T) {
		url := fmt.Sprintf(
			"ws://%s/piko/v1/listener/my-endpoint",
			upstreamLn.Addr().String(),
		)
		_, err := websocket.Dial(context.TODO(), url)
		require.ErrorContains(t, err, "bad handshake")
	})
}
