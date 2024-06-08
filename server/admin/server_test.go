package admin

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/testutil"
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
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := NewServer(
		prometheus.NewRegistry(),
		nil,
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

func TestServer_StatusRoutes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := NewServer(
		prometheus.NewRegistry(),
		nil,
		log.NewNopLogger(),
	)
	s.AddStatus("/mystatus", &fakeStatus{})

	go func() {
		require.NoError(t, s.Serve(ln))
	}()
	defer s.Shutdown(context.TODO())

	t.Run("status ok", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/status/mystatus/foo", ln.Addr().String())
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
		url := fmt.Sprintf("http://%s/status/notfound", ln.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestServer_TLS(t *testing.T) {
	rootCAPool, cert, err := testutil.LocalTLSServerCert()
	require.NoError(t, err)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	tlsConfig := &tls.Config{}
	tlsConfig.Certificates = []tls.Certificate{cert}

	s := NewServer(
		prometheus.NewRegistry(),
		tlsConfig,
		log.NewNopLogger(),
	)
	go func() {
		require.NoError(t, s.Serve(ln))
	}()
	defer s.Shutdown(context.TODO())

	t.Run("https ok", func(t *testing.T) {
		tlsConfig = &tls.Config{
			RootCAs: rootCAPool,
		}
		transport := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		client := &http.Client{
			Transport: transport,
		}

		req, _ := http.NewRequest(
			http.MethodGet,
			fmt.Sprintf("https://%s/health", ln.Addr().String()),
			nil,
		)
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("https bad ca", func(t *testing.T) {
		url := fmt.Sprintf("https://%s/health", ln.Addr().String())
		_, err := http.Get(url)
		assert.ErrorContains(t, err, "certificate signed by unknown authority")
	})

	t.Run("http", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/health", ln.Addr().String())
		resp, err := http.Get(url)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
