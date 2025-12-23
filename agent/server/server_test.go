package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dragonflydb/piko/pkg/log"
)

func TestServer_AdminRoutes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := NewServer(
		prometheus.NewRegistry(),
		log.NewNopLogger(),
	)
	go func() {
		require.NoError(t, s.Serve(ln))
	}()
	defer s.Shutdown(context.TODO())

	t.Run("metrics", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/metrics", ln.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("not found", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/foo", ln.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
