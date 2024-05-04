//go:build system

package tests

import (
	"context"
	"net/http"
	"testing"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server"
	serverconfig "github.com/andydunstall/pico/server/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_AdminAPI(t *testing.T) {
	serverConf := defaultServerConfig()
	server, err := server.NewServer(serverConf, log.NewNopLogger())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		require.NoError(t, server.Run(ctx))
	}()

	t.Run("health", func(t *testing.T) {
		resp, err := http.Get(
			"http://" + serverConf.Admin.AdvertiseAddr + "/health",
		)
		assert.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("metrics", func(t *testing.T) {
		resp, err := http.Get(
			"http://" + serverConf.Admin.AdvertiseAddr + "/metrics",
		)
		assert.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// defaultServerConfig returns the default server configuration for local
// tests.
func defaultServerConfig() *serverconfig.Config {
	return &serverconfig.Config{
		Proxy: serverconfig.ProxyConfig{
			BindAddr:       "127.0.0.1:0",
			GatewayTimeout: 1,
		},
		Upstream: serverconfig.UpstreamConfig{
			BindAddr: "127.0.0.1:0",
		},
		Admin: serverconfig.AdminConfig{
			BindAddr: "127.0.0.1:0",
		},
	}
}
