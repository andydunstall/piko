package client

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/andydunstall/yamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestListener returns a listener whose session runs over an in-memory
// pipe. Reconnect attempts are answered with a non-retryable status so
// connect returns instead of backing off indefinitely.
func newTestListener(t *testing.T) *listener {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	))
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	upstream := &Upstream{URL: u}
	ln := newListener("my-endpoint", upstream, upstream.logger())

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
	})

	muxConfig := yamux.DefaultConfig()
	muxConfig.Logger = nil
	muxConfig.LogOutput = io.Discard
	muxConfig.EnableKeepAlive = false

	sess, err := yamux.Client(clientConn, muxConfig)
	require.NoError(t, err)
	ln.sess = sess

	return ln
}

// Tests the listener reconnects when the session is closed by the server
// instead of by the listener, such as when a load balancer caps how long a
// connection may live.
func TestListener_AcceptReconnectsAfterRemoteClose(t *testing.T) {
	ln := newTestListener(t)

	require.NoError(t, ln.sess.Close())

	_, err := ln.Accept()
	require.Error(t, err)
	// ErrClosed means the listener gave up rather than reconnecting.
	assert.NotErrorIs(t, err, ErrClosed)
}

// Tests the listener stops accepting once it has been shut down.
func TestListener_AcceptReturnsErrClosedAfterShutdown(t *testing.T) {
	ln := newTestListener(t)

	require.NoError(t, ln.Shutdown())

	_, err := ln.Accept()
	assert.ErrorIs(t, err, ErrClosed)
}
