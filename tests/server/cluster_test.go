//go:build system

package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/andydunstall/piko/client"
	"github.com/andydunstall/piko/pikotest/cluster"
	"github.com/andydunstall/piko/pikotest/cluster/config"
	"github.com/andydunstall/piko/pikotest/cluster/proxy"
	"github.com/andydunstall/piko/pikotest/workload/upstreams"
	uconf "github.com/andydunstall/piko/pikotest/workload/upstreams/config"
	"github.com/andydunstall/piko/pkg/log"
)

// Tests proxying traffic across multiple Piko server nodes.
func TestCluster_Proxy(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		manager := cluster.NewManager()
		defer manager.Close()

		manager.Update(&config.Config{
			Nodes: 3,
		})

		remoteEndpointCh := make(chan string, 1)
		manager.Nodes()[1].ClusterState().OnRemoteEndpointUpdate(
			func(_ string, endpointID string) {
				remoteEndpointCh <- endpointID
			},
		)

		// Add upstream listener with a HTTP server returning 200.

		upstream := client.Upstream{
			URL: &url.URL{
				Scheme: "http",
				Host:   manager.Nodes()[0].UpstreamAddr(),
			},
		}
		ln, err := upstream.Listen(context.TODO(), "my-endpoint")
		assert.NoError(t, err)

		server := httptest.NewUnstartedServer(http.HandlerFunc(
			func(_ http.ResponseWriter, _ *http.Request) {},
		))
		server.Listener = ln
		go server.Start()
		defer server.Close()

		// Wait for node 2 to learn about the new upstream.
		assert.Equal(t, "my-endpoint", <-remoteEndpointCh)

		// Send a request to the upstream via Piko.

		req, _ := http.NewRequest(
			http.MethodGet,
			"http://"+manager.Nodes()[1].ProxyAddr(),
			nil,
		)
		req.Header.Add("x-piko-endpoint", "my-endpoint")
		httpClient := &http.Client{}
		resp, err := httpClient.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("tcp", func(t *testing.T) {
		manager := cluster.NewManager()
		defer manager.Close()

		manager.Update(&config.Config{
			Nodes: 3,
		})

		remoteEndpointCh := make(chan string, 1)
		manager.Nodes()[1].ClusterState().OnRemoteEndpointUpdate(
			func(_ string, endpointID string) {
				remoteEndpointCh <- endpointID
			},
		)

		// Create a client connecting to node 1 for the upstream listener and
		// node 2 for the proxy connection.
		upstream := client.Upstream{
			URL: &url.URL{
				Scheme: "http",
				Host:   manager.Nodes()[0].UpstreamAddr(),
			},
		}
		dialer := client.Dialer{
			URL: &url.URL{
				Scheme: "http",
				Host:   manager.Nodes()[1].ProxyAddr(),
			},
		}
		ln, err := upstream.Listen(context.TODO(), "my-endpoint")
		assert.NoError(t, err)

		// Wait for node 2 to learn about the new upstream.
		assert.Equal(t, "my-endpoint", <-remoteEndpointCh)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := ln.Accept()
			assert.NoError(t, err)

			// Echo server.
			buf := make([]byte, 512)
			for {
				n, err := conn.Read(buf)
				if err == io.EOF {
					return
				}
				assert.NoError(t, err)
				_, err = conn.Write(buf[:n])
				assert.NoError(t, err)
			}
		}()

		conn, err := dialer.Dial(context.TODO(), "my-endpoint")
		assert.NoError(t, err)

		// Test writing bytes to the upstream and waiting for them to be
		// echoed back.

		buf := make([]byte, 512)
		for i := 0; i != 1; i++ {
			_, err = conn.Write([]byte("foo"))
			assert.NoError(t, err)

			n, err := conn.Read(buf)
			assert.NoError(t, err)
			assert.Equal(t, 3, n)
		}

		// Verify closing the connection to Piko also closes the connection
		// to the upstream.
		conn.Close()
		wg.Wait()
	})
}

// Testing cluster upstream connection rebalancing
func TestCluster_Rebalancing(t *testing.T) {
	manager := cluster.NewManager()
	defer manager.Close()
	manager.Update(&config.Config{
		Nodes: 3,
	})
	loadBalancer := proxy.NewLoadBalancer(manager)
	defer loadBalancer.Close()
	conf := uconf.Default()
	logLevel := "error"
	logger, _ := log.NewLogger(logLevel, conf.Log.Subsystems)

	// Create 1000 upstream connections
	for i := 0; i < 1000; i++ {
		upstream, _ := upstreams.NewTCPUpstream("my-endpoint"+strconv.Itoa(i), conf, logger)
		defer upstream.Close()
	}

	getConnectionsCount := func(nodeIndex int) (string, int) {
		state := manager.Nodes()[nodeIndex].ClusterState()
		_, local := state.TotalAndLocalUpstreams()
		id := state.LocalID()
		return id, local
	}
	time.Sleep(2 * time.Second)

	initConns := make(map[string]int)
	for i := 0; i < 3; i++ {
		id, conns := getConnectionsCount(i)
		initConns[id] = conns
	}

	// Add two new nodes
	manager.Update(&config.Config{
		Nodes: 5,
	})

	// Waiting for a period of time to rebalance
	time.Sleep(15 * time.Second)

	for i := 0; i < 5; i++ {
		id, conns := getConnectionsCount(i)
		oldConns, ok := initConns[id]
		if !ok {
			// If it is a new node, conns should be greater than 0
			assert.Greater(t, conns, 0)
		} else {
			// If it is an old node, conns should be reduced
			assert.Less(t, conns, oldConns)
		}
	}
}
