package auth

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type PikoClaims struct {
	Endpoints []string `json:"endpoints"`
}

type JWTClaims struct {
	jwt.RegisteredClaims
	Piko PikoClaims `json:"piko"`
}

// JWTVerifier verifies client JWT tokens.
type JWTVerifier struct {
	hmacSecretKey  []byte
	rsaPublicKey   *rsa.PublicKey
	ecdsaPublicKey *ecdsa.PublicKey

	audience string
	issuer   string

	disableDisconnectOnExpiry bool

	// methods contains the valid JWT methods, which depends on the
	// verification keys configured.
	methods []string
}

func NewJWTVerifier(conf *LoadedConfig) *JWTVerifier {
	v := &JWTVerifier{
		audience:                  conf.Audience,
		issuer:                    conf.Issuer,
		disableDisconnectOnExpiry: conf.DisableDisconnectOnExpiry,
	}

	if len(conf.HMACSecretKey) > 0 {
		v.hmacSecretKey = conf.HMACSecretKey
		v.methods = append(v.methods, []string{"HS256", "HS384", "HS512"}...)
	}
	if conf.RSAPublicKey != nil {
		v.rsaPublicKey = conf.RSAPublicKey
		v.methods = append(v.methods, []string{"RS256", "RS384", "RS512"}...)
	}
	if conf.ECDSAPublicKey != nil {
		v.ecdsaPublicKey = conf.ECDSAPublicKey
		v.methods = append(v.methods, []string{"ES256", "ES384", "ES512"}...)
	}
	return v
}

func (v *JWTVerifier) Verify(tokenString string) (*Token, error) {
	claims := &JWTClaims{}

	opts := []jwt.ParserOption{
		jwt.WithValidMethods(v.methods),
	}
	if v.audience != "" {
		opts = append(opts, jwt.WithAudience(v.audience))
	}
	if v.issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.issuer))
	}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			switch token.Method.Alg() {
			case "HS256":
				fallthrough
			case "HS384":
				fallthrough
			case "HS512":
				return v.hmacSecretKey, nil
			case "RS256":
				fallthrough
			case "RS384":
				fallthrough
			case "RS512":
				return v.rsaPublicKey, nil
			case "ES256":
				fallthrough
			case "ES384":
				fallthrough
			case "ES512":
				return v.ecdsaPublicKey, nil
			default:
				return nil, fmt.Errorf("unsupported algorithm: %s", token.Method.Alg())
			}
		},
		opts...,
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// Discard the expiry if DisableDisconnectOnExpiry (we've already
	// checked whether the token expired, claims.ExpiresAt is used to
	// disconnect when the token expires).
	var expiry time.Time
	if claims.ExpiresAt != nil && !v.disableDisconnectOnExpiry {
		expiry = claims.ExpiresAt.Time
	}
	return &Token{
		Expiry:    expiry,
		Endpoints: claims.Piko.Endpoints,
	}, nil
}

var _ Verifier = &JWTVerifier{}
