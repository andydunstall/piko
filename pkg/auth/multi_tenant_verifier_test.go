package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestMultiTenantVerifier(t *testing.T) {
	defaultSecretKey := generateTestHSKey(t)
	tenant1SecretKey := generateTestHSKey(t)
	tenant2SecretKey := generateTestHSKey(t)

	endpointClaims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}

	defaultVerifier := NewJWTVerifier(&LoadedConfig{
		HMACSecretKey: defaultSecretKey,
	})
	tenantVerifiers := map[string]Verifier{
		"tenant-1": NewJWTVerifier(&LoadedConfig{
			HMACSecretKey: tenant1SecretKey,
		}),
		"tenant-2": NewJWTVerifier(&LoadedConfig{
			HMACSecretKey: tenant2SecretKey,
		}),
	}
	verifier := NewMultiTenantVerifier(defaultVerifier, tenantVerifiers)

	t.Run("default", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, endpointClaims)
		tokenString, err := token.SignedString([]byte(defaultSecretKey))
		assert.NoError(t, err)

		_, err = verifier.Verify(tokenString, "")
		assert.Equal(t, ErrUnknownTenant, err)
	})

	t.Run("tenant", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, endpointClaims)
		tokenString, err := token.SignedString([]byte(tenant1SecretKey))
		assert.NoError(t, err)

		parsedToken, err := verifier.Verify(tokenString, "tenant-1")
		assert.NoError(t, err)

		assert.Equal(t, parsedToken.TenantID, "tenant-1")
	})

	// Tests tenant 1 using tenants 2 key.
	t.Run("incorrect key", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, endpointClaims)
		tokenString, err := token.SignedString([]byte(tenant2SecretKey))
		assert.NoError(t, err)

		_, err = verifier.Verify(tokenString, "tenant-1")
		assert.Equal(t, ErrInvalidToken, err)
	})

	t.Run("unknown tenant", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, endpointClaims)
		tokenString, err := token.SignedString([]byte(defaultSecretKey))
		assert.NoError(t, err)

		_, err = verifier.Verify(tokenString, "unknown")
		assert.Equal(t, ErrUnknownTenant, err)
	})
}
