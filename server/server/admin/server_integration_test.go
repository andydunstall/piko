//go:build integration

package admin

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/cluster"
	"github.com/andydunstall/piko/server/status"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStatus struct {
}

func (s *fakeStatus) Register(group *gin.RouterGroup) {
	group.GET("/foo", s.fooRoute)
}

func (s *fakeStatus) fooRoute(c *gin.Context) {
	c.String(http.StatusOK, "foo")
}

var _ status.Handler = &fakeStatus{}

func TestServer_AdminRoutes(t *testing.T) {
	adminLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	adminServer := NewServer(
		adminLn,
		nil,
		prometheus.NewRegistry(),
		log.NewNopLogger(),
	)
	go func() {
		require.NoError(t, adminServer.Serve())
	}()
	defer adminServer.Shutdown(context.TODO())

	t.Run("health", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/health", adminLn.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("metrics", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/metrics", adminLn.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("not found", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/foo", adminLn.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestServer_StatusRoutes(t *testing.T) {
	adminLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	adminServer := NewServer(
		adminLn,
		nil,
		prometheus.NewRegistry(),
		log.NewNopLogger(),
	)
	adminServer.AddStatus("/mystatus", &fakeStatus{})

	go func() {
		require.NoError(t, adminServer.Serve())
	}()
	defer adminServer.Shutdown(context.TODO())

	t.Run("status ok", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/status/mystatus/foo", adminLn.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		buf := new(bytes.Buffer)
		//nolint
		buf.ReadFrom(resp.Body)
		assert.Equal(t, []byte("foo"), buf.Bytes())
	})

	t.Run("not found", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/status/notfound", adminLn.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// TestServer_ForwardRequest tests forwarding an admin request to another node
// in the cluster.
func TestServer_ForwardRequest(t *testing.T) {
	admin1Ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	cluster1State := cluster.NewState(&cluster.Node{
		ID:        "node-1",
		AdminAddr: admin1Ln.Addr().String(),
	}, log.NewNopLogger())

	admin1Server := NewServer(
		admin1Ln,
		cluster1State,
		prometheus.NewRegistry(),
		log.NewNopLogger(),
	)
	// Note only node 1 registers the status route.
	admin1Server.AddStatus("/mystatus", &fakeStatus{})

	go func() {
		require.NoError(t, admin1Server.Serve())
	}()
	defer admin1Server.Shutdown(context.TODO())

	admin2Ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	cluster2State := cluster.NewState(&cluster.Node{
		ID:        "node-2",
		AdminAddr: admin2Ln.Addr().String(),
	}, log.NewNopLogger())
	cluster2State.AddNode(&cluster.Node{
		ID:        "node-1",
		AdminAddr: admin1Ln.Addr().String(),
	})

	admin2Server := NewServer(
		admin2Ln,
		cluster2State,
		prometheus.NewRegistry(),
		log.NewNopLogger(),
	)

	go func() {
		require.NoError(t, admin2Server.Serve())
	}()
	defer admin2Server.Shutdown(context.TODO())

	t.Run("forward ok", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/status/mystatus/foo?forward=node-1", admin2Ln.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		buf := new(bytes.Buffer)
		//nolint
		buf.ReadFrom(resp.Body)
		assert.Equal(t, []byte("foo"), buf.Bytes())
	})

	t.Run("forward not found", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/status/mystatus/foo?forward=node-3", admin2Ln.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
