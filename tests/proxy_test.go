//go:build system

package tests

import (
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/andydunstall/piko/agent"
	agentconfig "github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server"
	statusclient "github.com/andydunstall/piko/status/client"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxy(t *testing.T) {
	serverConf := defaultServerConfig()
	server, err := server.NewServer(serverConf, log.NewNopLogger())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		require.NoError(t, server.Run(ctx))
	}()

	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {},
	))
	defer upstream.Close()

	agentConf := defaultAgentConfig(serverConf.Upstream.AdvertiseAddr)
	endpoint := agent.NewEndpoint(
		"my-endpoint",
		upstream.Listener.Addr().String(),
		agentConf,
		nil,
		agent.NewMetrics(),
		log.NewNopLogger(),
	)
	go func() {
		assert.NoError(t, endpoint.Run(ctx))
	}()

	// Wait for the agent to register the endpoint with Piko.
	for {
		statusClient := statusclient.NewClient(&url.URL{
			Scheme: "http",
			Host:   serverConf.Admin.AdvertiseAddr,
		}, "")
		endpoints, err := statusClient.ProxyEndpoints()
		assert.NoError(t, err)

		if len(endpoints) == 0 {
			<-time.After(time.Millisecond * 10)
			continue
		}

		_, ok := endpoints["my-endpoint"]
		assert.True(t, ok)
		break
	}

	// Send a request to Piko which should be forwarded to the upstream server.
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://"+serverConf.Proxy.AdvertiseAddr, nil)
	req.Header.Set("x-piko-endpoint", "my-endpoint")
	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestProxy_Authenticated(t *testing.T) {
	hsSecretKey := generateTestHSKey(t)

	serverConf := defaultServerConfig()
	serverConf.Auth.TokenHMACSecretKey = string(hsSecretKey)

	server, err := server.NewServer(serverConf, log.NewNopLogger())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		require.NoError(t, server.Run(ctx))
	}()

	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {},
	))
	defer upstream.Close()

	endpointClaims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		Issuer:    "bar",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, endpointClaims)
	apiKey, err := token.SignedString([]byte(hsSecretKey))
	assert.NoError(t, err)

	agentConf := defaultAgentConfig(serverConf.Upstream.AdvertiseAddr)
	agentConf.Auth.APIKey = apiKey
	endpoint := agent.NewEndpoint(
		"my-endpoint",
		upstream.Listener.Addr().String(),
		agentConf,
		nil,
		agent.NewMetrics(),
		log.NewNopLogger(),
	)
	go func() {
		assert.NoError(t, endpoint.Run(ctx))
	}()

	// Wait for the agent to register the endpoint with Piko.
	for {
		statusClient := statusclient.NewClient(&url.URL{
			Scheme: "http",
			Host:   serverConf.Admin.AdvertiseAddr,
		}, "")
		endpoints, err := statusClient.ProxyEndpoints()
		assert.NoError(t, err)

		if len(endpoints) == 0 {
			<-time.After(time.Millisecond * 10)
			continue
		}

		_, ok := endpoints["my-endpoint"]
		assert.True(t, ok)
		break
	}

	// Send a request to Piko which should be forwarded to the upstream server.
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://"+serverConf.Proxy.AdvertiseAddr, nil)
	req.Header.Set("x-piko-endpoint", "my-endpoint")
	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func defaultAgentConfig(serverAddr string) *agentconfig.Config {
	return &agentconfig.Config{
		Server: agentconfig.ServerConfig{
			URL:               "http://" + serverAddr,
			HeartbeatInterval: time.Second,
			HeartbeatTimeout:  time.Second,
		},
		Forwarder: agentconfig.ForwarderConfig{
			Timeout: time.Second,
		},
		Admin: agentconfig.AdminConfig{
			BindAddr: "127.0.0.1:0",
		},
	}
}

func generateTestHSKey(t *testing.T) []byte {
	b := make([]byte, 10)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}
