//go:build system

package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andydunstall/piko/agent/client"
	"github.com/andydunstall/piko/tests/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests proxying traffic across multiple Piko server nodes.
func TestCluster_Proxy(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		cluster, err := cluster.NewCluster()
		require.NoError(t, err)
		require.NoError(t, cluster.Start())
		defer cluster.Stop()

		remoteEndpointCh := make(chan string)
		cluster.Nodes()[1].ClusterState().OnRemoteEndpointUpdate(
			func(nodeID string, endpointID string) {
				remoteEndpointCh <- endpointID
			},
		)

		// Add upstream listener with a HTTP server returning 200.

		upstreamURL := "http://" + cluster.Nodes()[0].UpstreamAddr()
		pikoClient := client.New(client.WithURL(upstreamURL))
		ln, err := pikoClient.Listen(context.TODO(), "my-endpoint")
		assert.NoError(t, err)

		server := httptest.NewUnstartedServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {},
		))
		server.Listener = ln
		go server.Start()
		defer server.Close()

		// Wait for node 1 to learn about the new upstream.
		assert.Equal(t, "my-endpoint", <-remoteEndpointCh)

		// Send a request to the upstream via Piko.

		req, _ := http.NewRequest(
			http.MethodGet,
			"http://"+cluster.Nodes()[1].ProxyAddr(),
			nil,
		)
		req.Header.Add("x-piko-endpoint", "my-endpoint")
		httpClient := &http.Client{}
		resp, err := httpClient.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
