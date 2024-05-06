package auth

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type picoEndpointClaims struct {
	Endpoints []string `json:"endpoints"`
}

type endpointJWTClaims struct {
	jwt.RegisteredClaims
	Pico picoEndpointClaims `json:"pico"`
}

type JWTVerifierConfig struct {
	HMACSecretKey  []byte
	RSAPublicKey   *rsa.PublicKey
	ECDSAPublicKey *ecdsa.PublicKey
	Audience       string
	Issuer         string
}

type JWTVerifier struct {
	hmacSecretKey  []byte
	rsaPublicKey   *rsa.PublicKey
	ecdsaPublicKey *ecdsa.PublicKey

	audience string
	issuer   string

	// methods contains the valid JWT methods.
	methods []string
}

func NewJWTVerifier(conf JWTVerifierConfig) *JWTVerifier {
	v := &JWTVerifier{
		audience: conf.Audience,
		issuer:   conf.Issuer,
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

func (v *JWTVerifier) VerifyEndpointToken(tokenString string) (EndpointToken, error) {
	claims := &endpointJWTClaims{}

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
			return EndpointToken{}, ErrExpiredToken
		}
		return EndpointToken{}, ErrInvalidToken
	}
	if !token.Valid {
		return EndpointToken{}, ErrInvalidToken
	}

	var expiry time.Time
	if claims.ExpiresAt != nil {
		expiry = claims.ExpiresAt.Time
	}
	return EndpointToken{
		Expiry:    expiry,
		Endpoints: claims.Pico.Endpoints,
	}, nil
}

var _ Verifier = &JWTVerifier{}
