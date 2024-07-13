//go:build system

package tests

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"

	"github.com/andydunstall/piko/client"
	"github.com/andydunstall/piko/pikotest/cluster"
)

type errorMessage struct {
	Error string `json:"error"`
}

// Tests proxying HTTP traffic with a single Piko server node.
func TestProxy_HTTP(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		node := cluster.NewNode()
		node.Start()
		defer node.Stop()

		// Add upstream listener with a HTTP server returning 200.

		upstream := client.Upstream{
			URL: &url.URL{
				Scheme: "http",
				Host:   node.UpstreamAddr(),
			},
		}
		ln, err := upstream.Listen(context.TODO(), "my-endpoint")
		assert.NoError(t, err)

		server := httptest.NewUnstartedServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				// Note can't use io.Copy as not supported by http.ResponseWriter.
				b, err := io.ReadAll(r.Body)
				if err != nil {
					panic(fmt.Sprintf("read body: %s", err.Error()))
				}
				n, err := w.Write(b)
				if err != nil {
					panic(fmt.Sprintf("write bytes: %d: %s", n, err))
				}
			},
		))
		server.Listener = ln
		go server.Start()
		defer server.Close()

		// Send a request to the upstream via Piko.

		reqBody := randomBytes(4 * 1024)
		req, _ := http.NewRequest(
			http.MethodGet,
			"http://"+node.ProxyAddr(),
			bytes.NewReader(reqBody),
		)
		req.Header.Add("x-piko-endpoint", "my-endpoint")
		httpClient := &http.Client{}
		resp, err := httpClient.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, reqBody, respBody)
	})

	t.Run("https", func(t *testing.T) {
		node := cluster.NewNode(cluster.WithTLS(true))
		node.Start()
		defer node.Stop()

		clientTLSConfig := &tls.Config{
			RootCAs: node.RootCAPool(),
		}

		// Add upstream listener with a HTTP server returning 200.

		upstream := client.Upstream{
			URL: &url.URL{
				Scheme: "https",
				Host:   node.UpstreamAddr(),
			},
			TLSConfig: clientTLSConfig,
		}
		ln, err := upstream.Listen(context.TODO(), "my-endpoint")
		assert.NoError(t, err)

		server := httptest.NewUnstartedServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				// Note can't use io.Copy as not supported by http.ResponseWriter.
				b, err := io.ReadAll(r.Body)
				if err != nil {
					panic(fmt.Sprintf("read body: %s", err.Error()))
				}
				n, err := w.Write(b)
				if err != nil {
					panic(fmt.Sprintf("write bytes: %d: %s", n, err))
				}
			},
		))
		server.Listener = ln
		go server.Start()
		defer server.Close()

		// Send a request to the upstream via Piko.

		reqBody := randomBytes(4 * 1024)
		req, _ := http.NewRequest(
			http.MethodGet,
			"https://"+node.ProxyAddr(),
			bytes.NewReader(reqBody),
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

		respBody, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, reqBody, respBody)
	})

	t.Run("websocket", func(t *testing.T) {
		node := cluster.NewNode()
		node.Start()
		defer node.Stop()

		// Add upstream listener with a WebSocket server that echos back the
		// first message.

		upstream := client.Upstream{
			URL: &url.URL{
				Scheme: "http",
				Host:   node.UpstreamAddr(),
			},
		}
		ln, err := upstream.Listen(context.TODO(), "my-endpoint")
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
		node := cluster.NewNode()
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

func TestProxy_TCP(t *testing.T) {
	t.Run("tcp", func(t *testing.T) {
		node := cluster.NewNode()
		node.Start()
		defer node.Stop()

		upstream := client.Upstream{
			URL: &url.URL{
				Scheme: "http",
				Host:   node.UpstreamAddr(),
			},
		}
		ln, err := upstream.Listen(context.TODO(), "my-endpoint")
		assert.NoError(t, err)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			for {
				defer wg.Done()
				conn, err := ln.Accept()
				if errors.Is(err, client.ErrClosed) {
					return
				}
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
			}
		}()

		dialer := client.Dialer{
			URL: &url.URL{
				Scheme: "http",
				Host:   node.ProxyAddr(),
			},
		}
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

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic("read rand: " + err.Error())
	}
	return b
}
