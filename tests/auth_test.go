//go:build system

package tests

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/andydunstall/piko/agent/client"
	"github.com/andydunstall/piko/server/auth"
	"github.com/andydunstall/piko/workload/cluster"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pikoEndpointClaims struct {
	Endpoints []string `json:"endpoints"`
}

type endpointJWTClaims struct {
	jwt.RegisteredClaims
	Piko pikoEndpointClaims `json:"piko"`
}

// Tests upstream authentication.
func TestAuth_Upstream(t *testing.T) {
	endpointClaims := endpointJWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Piko: pikoEndpointClaims{
			Endpoints: []string{"my-endpoint"},
		},
	}

	// Tests an upstream authenticating with a valid token.
	t.Run("valid", func(t *testing.T) {
		secretKey := generateTestHSKey()
		node, err := cluster.NewNode("my-node", cluster.WithVerifierConfig(&auth.JWTVerifierConfig{
			HMACSecretKey: secretKey,
		}))
		require.NoError(t, err)
		node.Start()
		defer node.Stop()

		token := jwt.NewWithClaims(jwt.SigningMethodHS512, endpointClaims)
		tokenString, err := token.SignedString([]byte(secretKey))
		assert.NoError(t, err)

		upstreamURL := "http://" + node.UpstreamAddr()
		pikoClient := client.New(
			client.WithUpstreamURL(upstreamURL),
			client.WithToken(tokenString),
		)
		ln, err := pikoClient.Listen(context.TODO(), "my-endpoint")
		assert.NoError(t, err)
		defer ln.Close()
	})

	// Tests an upstream authenticating with an invalid token (signed by
	// the wrong key).
	t.Run("invalid", func(t *testing.T) {
		secretKey := generateTestHSKey()
		node, err := cluster.NewNode("my-node", cluster.WithVerifierConfig(&auth.JWTVerifierConfig{
			HMACSecretKey: secretKey,
		}))
		require.NoError(t, err)
		node.Start()
		defer node.Stop()

		token := jwt.NewWithClaims(jwt.SigningMethodHS512, endpointClaims)
		tokenString, err := token.SignedString([]byte("invalid-key"))
		assert.NoError(t, err)

		upstreamURL := "http://" + node.UpstreamAddr()
		pikoClient := client.New(
			client.WithUpstreamURL(upstreamURL),
			client.WithToken(tokenString),
		)
		_, err = pikoClient.Listen(context.TODO(), "my-endpoint")
		assert.ErrorContains(t, err, "connect: 401: invalid token")
	})

	// Tests an unauthenticated upstream attempting to connect.
	t.Run("unauthenticated", func(t *testing.T) {
		node, err := cluster.NewNode("my-node", cluster.WithVerifierConfig(&auth.JWTVerifierConfig{
			HMACSecretKey: generateTestHSKey(),
		}))
		require.NoError(t, err)
		node.Start()
		defer node.Stop()

		upstreamURL := "http://" + node.UpstreamAddr()
		pikoClient := client.New(client.WithUpstreamURL(upstreamURL))
		_, err = pikoClient.Listen(context.TODO(), "my-endpoint")
		assert.ErrorContains(t, err, "connect: 401: missing authorization")
	})
}

func generateTestHSKey() []byte {
	b := make([]byte, 10)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}
