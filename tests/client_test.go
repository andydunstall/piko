//go:build system

package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/andydunstall/piko/client"
	cluster "github.com/andydunstall/piko/workloadv2/cluster"
)

// TestClient_ListenAndForward tests forwarding incoming connections to a local
// HTTP server.
func TestClient_ListenAndForward(t *testing.T) {
	node := cluster.NewNode()
	node.Start()
	defer node.Stop()

	server := httptest.NewServer(http.HandlerFunc(
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
	defer server.Close()

	// Add upstream listener with a HTTP server returning 200.

	upstream := client.Upstream{
		URL: &url.URL{
			Scheme: "http",
			Host:   node.UpstreamAddr(),
		},
	}

	forwarder, err := upstream.ListenAndForward(
		context.Background(), "my-endpoint", server.Listener.Addr().String(),
	)
	assert.NoError(t, err)

	// Send requests to the upstream via Piko.

	reqBody := randomBytes(4 * 1024)
	for i := 0; i != 5; i++ {
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
	}

	assert.NoError(t, forwarder.Close())
	assert.NoError(t, forwarder.Wait())
}
