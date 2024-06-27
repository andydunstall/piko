package auth

import (
	"github.com/spf13/pflag"
)

type Config struct {
	// TokenHMACSecretKey is the secret key to authenticate HMAC endpoint
	// connection JWTs.
	TokenHMACSecretKey string `json:"token_hmac_secret_key" yaml:"token_hmac_secret_key"`

	// TokenRSAPublicKey is the public key to authenticate RSA endpoint
	// connection JWTs.
	TokenRSAPublicKey string `json:"token_rsa_public_key" yaml:"token_rsa_public_key"`

	// TokenECDSAPublicKey is the public key to authenticate ECDSA endpoint
	// connection JWTs.
	TokenECDSAPublicKey string `json:"token_ecdsa_public_key" yaml:"token_ecdsa_public_key"`

	// TokenAudience is the required 'aud' claim of the authenticated JWTs.
	//
	// If not given the 'aud' claim will be ignored.
	TokenAudience string `json:"token_audience" yaml:"token_audience"`

	// TokenIssuer is the required 'iss' claim of the authenticated JWTs.
	//
	// If not given the 'iss' claim will be ignored.
	TokenIssuer string `json:"token_issuer" yaml:"token_issuer"`
}

func (c *Config) AuthEnabled() bool {
	return c.TokenHMACSecretKey != "" || c.TokenRSAPublicKey != "" || c.TokenECDSAPublicKey != ""
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.TokenHMACSecretKey,
		"auth.token-hmac-secret-key",
		"",
		`
Secret key to authenticate HMAC endpoint connection JWTs.`,
	)
	fs.StringVar(
		&c.TokenRSAPublicKey,
		"auth.token-rsa-public-key",
		"",
		`
Public key to authenticate RSA endpoint connection JWTs.`,
	)
	fs.StringVar(
		&c.TokenECDSAPublicKey,
		"auth.token-ecdsa-public-key",
		"",
		`
Public key to authenticate ECDSA endpoint connection JWTs.`,
	)
	fs.StringVar(
		&c.TokenAudience,
		"auth.token-audience",
		"",
		`
Audience of endpoint connection JWT token to verify.

If given the JWT 'aud' claim must match the given audience. Otherwise it
is ignored.`,
	)
	fs.StringVar(
		&c.TokenIssuer,
		"auth.token-issuer",
		"",
		`
Issuer of endpoint connection JWT token to verify.

If given the JWT 'iss' claim must match the given issuer. Otherwise it
is ignored.`,
	)
}
