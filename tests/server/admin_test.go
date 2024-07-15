//go:build system

package server

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	cluster "github.com/andydunstall/piko/pikotest/cluster"
	"github.com/andydunstall/piko/pkg/auth"
)

// Tests the admin server.
func TestAdmin(t *testing.T) {
	// Tests /health returns 200.
	t.Run("health", func(t *testing.T) {
		node := cluster.NewNode()
		node.Start()
		defer node.Stop()

		resp, err := http.Get("http://" + node.AdminAddr() + "/health")
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// Tests /ready returns 200.
	t.Run("ready", func(t *testing.T) {
		node := cluster.NewNode()
		node.Start()
		defer node.Stop()

		resp, err := http.Get("http://" + node.AdminAddr() + "/ready")
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// Tests /metrics returns 200.
	t.Run("metrics", func(t *testing.T) {
		node := cluster.NewNode()
		node.Start()
		defer node.Stop()

		resp, err := http.Get("http://" + node.AdminAddr() + "/metrics")
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// Tests admin server authentication.
func TestAdmin_Auth(t *testing.T) {
	endpointClaims := auth.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Piko: auth.PikoClaims{
			Endpoints: []string{"my-endpoint"},
		},
	}

	// Tests a client authenticating with a valid token.
	t.Run("valid", func(t *testing.T) {
		secretKey := generateTestHSKey()
		node := cluster.NewNode(cluster.WithAuthConfig(auth.Config{
			HMACSecretKey: string(secretKey),
		}))
		node.Start()
		defer node.Stop()

		token := jwt.NewWithClaims(jwt.SigningMethodHS512, endpointClaims)
		tokenString, err := token.SignedString([]byte(secretKey))
		assert.NoError(t, err)

		url := fmt.Sprintf("http://%s/health", node.AdminAddr())
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Add("Authorization", "Bearer "+tokenString)

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// Tests an upstream authenticating with an invalid token (signed by
	// the wrong key).
	t.Run("invalid", func(t *testing.T) {
		secretKey := generateTestHSKey()
		node := cluster.NewNode(cluster.WithAuthConfig(auth.Config{
			HMACSecretKey: string(secretKey),
		}))
		node.Start()
		defer node.Stop()

		token := jwt.NewWithClaims(jwt.SigningMethodHS512, endpointClaims)
		tokenString, err := token.SignedString([]byte("invalid-key"))
		assert.NoError(t, err)

		url := fmt.Sprintf("http://%s/health", node.AdminAddr())
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Add("Authorization", "Bearer "+tokenString)

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	// Tests an unauthenticated client attempting to connect.
	t.Run("unauthenticated", func(t *testing.T) {
		secretKey := generateTestHSKey()
		node := cluster.NewNode(cluster.WithAuthConfig(auth.Config{
			HMACSecretKey: string(secretKey),
		}))
		node.Start()
		defer node.Stop()

		url := fmt.Sprintf("http://%s/health", node.AdminAddr())
		req, _ := http.NewRequest(http.MethodGet, url, nil)

		client := &http.Client{}
		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
