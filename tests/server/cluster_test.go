//go:build system

package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dragonflydb/piko/client"
	"github.com/dragonflydb/piko/pikotest/cluster"
	"github.com/dragonflydb/piko/pikotest/cluster/config"
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
