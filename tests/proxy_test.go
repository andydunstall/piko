//go:build system

package tests

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andydunstall/piko/agent/client"
	"github.com/andydunstall/piko/tests/cluster"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errorMessage struct {
	Error string `json:"error"`
}

// Tests proxying HTTP traffic with a single Piko server node.
func TestProxy_HTTP(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		node, err := cluster.NewNode("my-node")
		require.NoError(t, err)
		node.Start()
		defer node.Stop()

		// Add upstream listener with a HTTP server returning 200.

		upstreamURL := "http://" + node.UpstreamAddr()
		pikoClient := client.New(client.WithURL(upstreamURL))
		ln, err := pikoClient.Listen(context.TODO(), "my-endpoint")
		assert.NoError(t, err)

		server := httptest.NewUnstartedServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {},
		))
		server.Listener = ln
		go server.Start()
		defer server.Close()

		// Send a request to the upstream via Piko.

		req, _ := http.NewRequest(
			http.MethodGet,
			"http://"+node.ProxyAddr(),
			nil,
		)
		req.Header.Add("x-piko-endpoint", "my-endpoint")
		httpClient := &http.Client{}
		resp, err := httpClient.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("https", func(t *testing.T) {
		node, err := cluster.NewNode("my-node", cluster.WithTLS(true))
		require.NoError(t, err)
		node.Start()
		defer node.Stop()

		clientTLSConfig := &tls.Config{
			RootCAs: node.RootCAPool(),
		}

		// Add upstream listener with a HTTP server returning 200.

		upstreamURL := "https://" + node.UpstreamAddr()
		pikoClient := client.New(
			client.WithURL(upstreamURL),
			client.WithTLSConfig(clientTLSConfig),
		)
		ln, err := pikoClient.Listen(context.TODO(), "my-endpoint")
		assert.NoError(t, err)

		server := httptest.NewUnstartedServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {},
		))
		server.Listener = ln
		go server.Start()
		defer server.Close()

		// Send a request to the upstream via Piko.

		req, _ := http.NewRequest(
			http.MethodGet,
			"https://"+node.ProxyAddr(),
			nil,
		)
		req.Header.Add("x-piko-endpoint", "my-endpoint")
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: clientTLSConfig,
			},
		}
		resp, err := httpClient.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("websocket", func(t *testing.T) {
		node, err := cluster.NewNode("my-node")
		require.NoError(t, err)
		node.Start()
		defer node.Stop()

		// Add upstream listener with a WebSocket server that echos back the
		// first message.

		upstreamURL := "http://" + node.UpstreamAddr()
		pikoClient := client.New(client.WithURL(upstreamURL))
		ln, err := pikoClient.Listen(context.TODO(), "my-endpoint")
		assert.NoError(t, err)

		server := httptest.NewUnstartedServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				var upgrader = websocket.Upgrader{}

				c, err := upgrader.Upgrade(w, r, nil)
				assert.NoError(t, err)
				defer c.Close()
				mt, message, err := c.ReadMessage()
				assert.NoError(t, err)

				assert.NoError(t, c.WriteMessage(mt, message))
			},
		))
		server.Listener = ln
		go server.Start()
		defer server.Close()

		// Send a WebSocket message via Piko and wait for it to be echoed back.

		header := make(http.Header)
		header.Add("x-piko-endpoint", "my-endpoint")

		c, _, err := websocket.DefaultDialer.Dial("ws://"+node.ProxyAddr(), header)
		assert.NoError(t, err)
		defer c.Close()

		assert.NoError(t, c.WriteMessage(websocket.TextMessage, []byte("echo")))

		mt, message, err := c.ReadMessage()
		assert.NoError(t, err)

		assert.Equal(t, websocket.TextMessage, mt)
		assert.Equal(t, []byte("echo"), message)
	})

	// Tests sending a request to an endpoint with no listeners.
	t.Run("no listeners", func(t *testing.T) {
		node, err := cluster.NewNode("my-node")
		require.NoError(t, err)
		node.Start()
		defer node.Stop()

		// Send a request to endpoint 'my-endpoint' with no upstream listeners.

		req, _ := http.NewRequest(
			http.MethodGet,
			"http://"+node.ProxyAddr(),
			nil,
		)
		req.Header.Add("x-piko-endpoint", "my-endpoint")
		httpClient := &http.Client{}
		resp, err := httpClient.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
		m := errorMessage{}
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&m))
		assert.Equal(t, "no available upstreams", m.Error)
	})
}
