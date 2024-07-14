package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTVerifier_HS(t *testing.T) {
	secretKey := generateTestHSKey(t)

	endpointClaims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Piko: PikoClaims{
			Endpoints: []string{"my-endpoint"},
		},
	}

	t.Run("valid", func(t *testing.T) {
		methods := []*jwt.SigningMethodHMAC{
			jwt.SigningMethodHS256,
			jwt.SigningMethodHS384,
			jwt.SigningMethodHS512,
		}
		for _, method := range methods {
			t.Run(method.Alg(), func(t *testing.T) {
				token := jwt.NewWithClaims(method, endpointClaims)
				tokenString, err := token.SignedString([]byte(secretKey))
				assert.NoError(t, err)

				verifier := NewJWTVerifier(&LoadedConfig{
					HMACSecretKey: secretKey,
				})
				parsedToken, err := verifier.Verify(tokenString)
				assert.NoError(t, err)

				assert.Equal(t, []string{"my-endpoint"}, parsedToken.Endpoints)
				assert.Equal(t, endpointClaims.ExpiresAt.Unix(), parsedToken.Expiry.Unix())
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, endpointClaims)
		tokenString, err := token.SignedString([]byte(secretKey))
		assert.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			HMACSecretKey: []byte("invalid key"),
		})
		assert.NoError(t, err)
		_, err = verifier.Verify(tokenString)
		assert.Equal(t, ErrInvalidToken, err)
	})
}

func TestJWTVerifier_RS(t *testing.T) {
	endpointClaims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Piko: PikoClaims{
			Endpoints: []string{"my-endpoint"},
		},
	}

	privateKey, publicKey := generateTestRSAKeys(t)
	t.Run("valid", func(t *testing.T) {
		methods := []*jwt.SigningMethodRSA{
			jwt.SigningMethodRS256,
			jwt.SigningMethodRS384,
			jwt.SigningMethodRS512,
		}
		for _, method := range methods {
			t.Run(method.Alg(), func(t *testing.T) {
				token := jwt.NewWithClaims(method, endpointClaims)
				tokenString, err := token.SignedString(privateKey)
				assert.NoError(t, err)

				verifier := NewJWTVerifier(&LoadedConfig{
					RSAPublicKey: publicKey,
				})
				parsedToken, err := verifier.Verify(tokenString)
				assert.NoError(t, err)

				assert.Equal(t, []string{"my-endpoint"}, parsedToken.Endpoints)
				assert.Equal(t, endpointClaims.ExpiresAt.Unix(), parsedToken.Expiry.Unix())
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		invalidPrivateKey, _ := generateTestRSAKeys(t)

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, endpointClaims)
		tokenString, err := token.SignedString(invalidPrivateKey)
		assert.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			RSAPublicKey: publicKey,
		})
		assert.NoError(t, err)
		_, err = verifier.Verify(tokenString)
		assert.Equal(t, ErrInvalidToken, err)
	})
}

func TestJWTVerifier_EC(t *testing.T) {
	endpointClaims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Piko: PikoClaims{
			Endpoints: []string{"my-endpoint"},
		},
	}

	t.Run("valid", func(t *testing.T) {
		tests := []struct {
			method *jwt.SigningMethodECDSA
			curve  elliptic.Curve
		}{
			{
				method: jwt.SigningMethodES256,
				curve:  elliptic.P256(),
			},
			{
				method: jwt.SigningMethodES384,
				curve:  elliptic.P384(),
			},
			{
				method: jwt.SigningMethodES512,
				curve:  elliptic.P521(),
			},
		}
		for _, tt := range tests {
			t.Run(tt.method.Alg(), func(t *testing.T) {
				privateKey, publicKey := generateTestECDSAKeys(tt.curve, t)

				token := jwt.NewWithClaims(tt.method, endpointClaims)
				tokenString, err := token.SignedString(privateKey)
				assert.NoError(t, err)

				verifier := NewJWTVerifier(&LoadedConfig{
					ECDSAPublicKey: publicKey,
				})
				parsedToken, err := verifier.Verify(tokenString)
				assert.NoError(t, err)

				assert.Equal(t, []string{"my-endpoint"}, parsedToken.Endpoints)
				assert.Equal(t, endpointClaims.ExpiresAt.Unix(), parsedToken.Expiry.Unix())
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, publicKey := generateTestECDSAKeys(elliptic.P256(), t)
		invalidPrivateKey, _ := generateTestECDSAKeys(elliptic.P256(), t)

		token := jwt.NewWithClaims(jwt.SigningMethodES256, endpointClaims)
		tokenString, err := token.SignedString(invalidPrivateKey)
		assert.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			ECDSAPublicKey: publicKey,
		})
		assert.NoError(t, err)
		_, err = verifier.Verify(tokenString)
		assert.Equal(t, ErrInvalidToken, err)
	})
}

func TestJWTVerifier_Invalid(t *testing.T) {
	t.Run("expired", func(t *testing.T) {
		secretKey := generateTestHSKey(t)

		endpointClaims := JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				// Expires in the past.
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			},
			Piko: PikoClaims{
				Endpoints: []string{"my-endpoint"},
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, endpointClaims)
		tokenString, err := token.SignedString([]byte(secretKey))
		assert.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			HMACSecretKey: secretKey,
		})
		_, err = verifier.Verify(tokenString)
		assert.Error(t, err)
	})

	t.Run("audience", func(t *testing.T) {
		secretKey := generateTestHSKey(t)

		endpointClaims := JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				Audience:  jwt.ClaimStrings([]string{"bar"}),
			},
			Piko: PikoClaims{
				Endpoints: []string{"my-endpoint"},
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, endpointClaims)
		tokenString, err := token.SignedString([]byte(secretKey))
		assert.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			HMACSecretKey: secretKey,
			Audience:      "foo",
		})
		_, err = verifier.Verify(tokenString)
		assert.Error(t, err)
	})

	t.Run("issuer", func(t *testing.T) {
		secretKey := generateTestHSKey(t)

		endpointClaims := JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				Issuer:    "bar",
			},
			Piko: PikoClaims{
				Endpoints: []string{"my-endpoint"},
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, endpointClaims)
		tokenString, err := token.SignedString([]byte(secretKey))
		assert.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			HMACSecretKey: secretKey,
			Issuer:        "foo",
		})
		_, err = verifier.Verify(tokenString)
		assert.Error(t, err)
	})
}

func generateTestHSKey(t *testing.T) []byte {
	b := make([]byte, 10)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

func generateTestRSAKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	reader := rand.Reader
	bitSize := 2048
	key, err := rsa.GenerateKey(reader, bitSize)
	require.NoError(t, err)
	return key, &key.PublicKey
}

func generateTestECDSAKeys(c elliptic.Curve, t *testing.T) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	reader := rand.Reader
	key, err := ecdsa.GenerateKey(c, reader)
	require.NoError(t, err)
	return key, &key.PublicKey
}
