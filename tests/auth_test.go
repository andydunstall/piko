//go:build system

package tests

import (
	"context"
	"crypto/rand"
	"net/url"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	"github.com/andydunstall/piko/client"
	"github.com/andydunstall/piko/pikotest/cluster"
	"github.com/andydunstall/piko/server/auth"
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
		node := cluster.NewNode(cluster.WithAuthConfig(auth.Config{
			TokenHMACSecretKey: string(secretKey),
		}))
		node.Start()
		defer node.Stop()

		token := jwt.NewWithClaims(jwt.SigningMethodHS512, endpointClaims)
		tokenString, err := token.SignedString([]byte(secretKey))
		assert.NoError(t, err)

		upstream := client.Upstream{
			URL: &url.URL{
				Scheme: "http",
				Host:   node.UpstreamAddr(),
			},
			Token: tokenString,
		}
		ln, err := upstream.Listen(context.TODO(), "my-endpoint")
		assert.NoError(t, err)
		defer ln.Close()
	})

	// Tests an upstream authenticating with an invalid token (signed by
	// the wrong key).
	t.Run("invalid", func(t *testing.T) {
		secretKey := generateTestHSKey()
		node := cluster.NewNode(cluster.WithAuthConfig(auth.Config{
			TokenHMACSecretKey: string(secretKey),
		}))
		node.Start()
		defer node.Stop()

		token := jwt.NewWithClaims(jwt.SigningMethodHS512, endpointClaims)
		tokenString, err := token.SignedString([]byte("invalid-key"))
		assert.NoError(t, err)

		upstream := client.Upstream{
			URL: &url.URL{
				Scheme: "http",
				Host:   node.UpstreamAddr(),
			},
			Token: tokenString,
		}
		_, err = upstream.Listen(context.TODO(), "my-endpoint")
		assert.ErrorContains(t, err, "connect: 401: invalid token")
	})

	// Tests an unauthenticated upstream attempting to connect.
	t.Run("unauthenticated", func(t *testing.T) {
		node := cluster.NewNode(cluster.WithAuthConfig(auth.Config{
			TokenHMACSecretKey: string(generateTestHSKey()),
		}))
		node.Start()
		defer node.Stop()

		upstream := client.Upstream{
			URL: &url.URL{
				Scheme: "http",
				Host:   node.UpstreamAddr(),
			},
		}
		_, err := upstream.Listen(context.TODO(), "my-endpoint")
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
