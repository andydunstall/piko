package auth

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/pflag"
)

type JWKSConfig struct {
	// Endpoint to load the JWKS from.
	//
	// Supports schemes http, https or file.
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// How long to cache the JWKS for before reloading.
	CacheTTL time.Duration `json:"cache_ttl" yaml:"cache_ttl"`

	// Timeout for loading the JWKS.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// Load will ensure that the correct KeyFunc is loaded
// and available as part of the returned LoadedJWKS pointer.
func (j *JWKSConfig) Load(ctx context.Context) (*LoadedJWKS, error) {
	endpoint, err := url.Parse(j.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint %q: %w", j.Endpoint, err)
	}

	switch endpoint.Scheme {
	case "http", "https":
		return j.loadRemote(ctx)
	case "file":
		return j.loadLocal(endpoint.Path)
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", endpoint.Scheme)
	}
}

// loadLocal will attempt to load the file containing the JWK Set from the local disk.
func (j *JWKSConfig) loadLocal(path string) (*LoadedJWKS, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", path, err)
	}

	kFunc, err := keyfunc.NewJWKSetJSON(contents)
	if err != nil {
		return nil, fmt.Errorf("parse JWKS: %w", err)
	}

	return &LoadedJWKS{
		KeyFunc: kFunc.Keyfunc,
	}, nil
}

// loadRemote will load the contents of the JWK Set from a remote endpoint.
// It will also ensure that the endpoint is scanned from time to time (`CacheTTL`)
// to ensure new tokens are visible.
func (j *JWKSConfig) loadRemote(ctx context.Context) (*LoadedJWKS, error) {
	kFunc, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{j.Endpoint}, keyfunc.Override{
		RefreshInterval: j.CacheTTL,
		HTTPTimeout:     j.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("load JWKS: %w", err)
	}

	return &LoadedJWKS{
		KeyFunc: kFunc.Keyfunc,
	}, nil
}

func (j *JWKSConfig) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	prefix += "jwks."

	fs.StringVar(
		&j.Endpoint,
		prefix+"endpoint",
		j.Endpoint,
		`
Endpoint to load the JWK Set from. Accepts remote endpoints or local paths`,
	)

	fs.DurationVar(
		&j.CacheTTL,
		prefix+"cache-ttl",
		j.CacheTTL,
		`
Frequency to refresh the JWK Set from the remote endpoint.`,
	)
}

// LoadedJWKS provides a ready to use jwt.KeyFunc for token verification.
type LoadedJWKS struct {
	// KeyFunc is the key function to use for verifying JWTs.
	KeyFunc jwt.Keyfunc
}
