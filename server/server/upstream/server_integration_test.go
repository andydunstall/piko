//go:build integration

package server

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/andydunstall/pico/pkg/conn"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/pkg/rpc"
	"github.com/prometheus/client_golang/prometheus"
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

func (p *fakeProxy) AddUpstream(endpointID string, _ rpc.Stream) {
	p.addUpstreamCh <- endpointID
}

func (p *fakeProxy) RemoveUpstream(endpointID string, _ rpc.Stream) {
	p.removeUpstreamCh <- endpointID
}

func TestServer_AddUpstream(t *testing.T) {
	upstreamLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	proxy := newFakeProxy()
	upstreamServer := NewServer(
		upstreamLn,
		proxy,
		prometheus.NewRegistry(),
		log.NewNopLogger(),
	)
	go func() {
		require.NoError(t, upstreamServer.Serve())
	}()
	defer upstreamServer.Shutdown(context.TODO())

	url := fmt.Sprintf(
		"ws://%s/pico/v1/listener/my-endpoint",
		upstreamLn.Addr().String(),
	)
	rpcServer := newRPCServer()
	conn, err := conn.DialWebsocket(context.TODO(), url)
	require.NoError(t, err)

	// Add client stream and ensure upstream added to proxy.
	stream := rpc.NewStream(conn, rpcServer.Handler(), log.NewNopLogger())
	assert.Equal(t, "my-endpoint", <-proxy.addUpstreamCh)

	// Close client stream and ensure upstream removed from proxy.
	stream.Close()
	assert.Equal(t, "my-endpoint", <-proxy.removeUpstreamCh)
}
