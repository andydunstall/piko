//go:build integration

package admin

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/status"
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
