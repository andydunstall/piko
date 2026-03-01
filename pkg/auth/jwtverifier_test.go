package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/go-jose/go-jose/v4"
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
		_, err = verifier.Verify(tokenString)
		assert.Equal(t, ErrInvalidToken, err)
	})
}

func TestJWTVerifier_JWKS(t *testing.T) {
	endpointClaims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Piko: PikoClaims{
			Endpoints: []string{"my-endpoint"},
		},
	}

	privateKey, kid, keyFunc := generateTestJWKS(t)

	t.Run("valid", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, endpointClaims)
		token.Header["kid"] = kid
		tokenString, err := token.SignedString(privateKey)

		assert.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			JWKS: &LoadedJWKS{
				KeyFunc: keyFunc,
			},
		})
		parsedToken, err := verifier.Verify(tokenString)
		require.NoError(t, err)

		assert.Equal(t, []string{"my-endpoint"}, parsedToken.Endpoints)
		assert.Equal(t, endpointClaims.ExpiresAt.Unix(), parsedToken.Expiry.Unix())
	})

	t.Run("unknown kid", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, endpointClaims)
		token.Header["kid"] = "unknown"
		tokenString, err := token.SignedString(privateKey)

		assert.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			JWKS: &LoadedJWKS{
				KeyFunc: keyFunc,
			},
		})
		_, err = verifier.Verify(tokenString)
		assert.Equal(t, ErrInvalidToken, err)
	})

	t.Run("invalid key", func(t *testing.T) {
		invalidPrivateKey, _ := generateTestRSAKeys(t)

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, endpointClaims)
		token.Header["kid"] = kid
		tokenString, err := token.SignedString(invalidPrivateKey)
		require.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			JWKS: &LoadedJWKS{
				KeyFunc: keyFunc,
			},
		})
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

func TestJWTVerifier_DisableDisconnectOnExpiry(t *testing.T) {
	secretKey := generateTestHSKey(t)

	endpointClaims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Piko: PikoClaims{
			Endpoints: []string{"my-endpoint"},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, endpointClaims)
	tokenString, err := token.SignedString([]byte(secretKey))
	assert.NoError(t, err)

	verifier := NewJWTVerifier(&LoadedConfig{
		HMACSecretKey:             secretKey,
		DisableDisconnectOnExpiry: true,
	})
	parsedToken, err := verifier.Verify(tokenString)
	assert.NoError(t, err)

	assert.Equal(t, []string{"my-endpoint"}, parsedToken.Endpoints)
	// The token expiry should not be set.
	assert.True(t, parsedToken.Expiry.IsZero())
}

func TestJWTVerifier_RequireEndpoints(t *testing.T) {
	secretKey := generateTestHSKey(t)

	t.Run("endpoints present passes", func(t *testing.T) {
		claims := JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			Piko: PikoClaims{
				Endpoints: []string{"my-endpoint"},
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(secretKey))
		require.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			HMACSecretKey:    secretKey,
			RequireEndpoints: true,
		})
		parsedToken, err := verifier.Verify(tokenString)
		assert.NoError(t, err)
		assert.Equal(t, []string{"my-endpoint"}, parsedToken.Endpoints)
	})

	t.Run("endpoints absent rejected when required", func(t *testing.T) {
		claims := JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(secretKey))
		require.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			HMACSecretKey:    secretKey,
			RequireEndpoints: true,
		})
		_, err = verifier.Verify(tokenString)
		assert.Equal(t, ErrInvalidToken, err)
	})

	t.Run("endpoints absent allowed when not required", func(t *testing.T) {
		claims := JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(secretKey))
		require.NoError(t, err)

		verifier := NewJWTVerifier(&LoadedConfig{
			HMACSecretKey: secretKey,
		})
		parsedToken, err := verifier.Verify(tokenString)
		assert.NoError(t, err)
		assert.Empty(t, parsedToken.Endpoints)
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

func generateTestJWKS(t *testing.T) (*rsa.PrivateKey, string, jwt.Keyfunc) {
	priv, _ := generateTestRSAKeys(t)

	privJWK := jose.JSONWebKey{
		Key:       priv,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}

	thumb, err := privJWK.Thumbprint(crypto.SHA256)
	require.NoError(t, err)
	privJWK.KeyID = base64.RawURLEncoding.EncodeToString(thumb)

	pubJWK := privJWK.Public()

	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{pubJWK},
	}

	json, err := json.Marshal(jwks)
	require.NoError(t, err)

	keyFunc, err := keyfunc.NewJWKSetJSON(json)
	require.NoError(t, err)

	return priv, privJWK.KeyID, keyFunc.Keyfunc
}
