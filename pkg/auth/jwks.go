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

type JWKS struct {
	// Endpoint to load the JWKS from.
	//
	// Supports schemes http, https or file.
	Endpoint string `json:"endpoint" yaml:"endpoint"`

	// How long to cache the JWKS for before reloading.
	CacheTTL time.Duration `json:"cache_ttl" yaml:"cache_ttl"`

	// Timeout for loading the JWKS.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// LoadedJWKS provides a ready to use jwt.KeyFunc for token verification.
type LoadedJWKS struct {
	// KeyFunc is the key function to use for verifying JWTs.
	KeyFunc jwt.Keyfunc
}

// Load will ensure that the correct KeyFunc is loaded
// and available as part of the returned LoadedJWKS pointer.
func (j *JWKS) Load(ctx context.Context) (*LoadedJWKS, error) {
	endpoint, err := url.Parse(j.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWKS endpoint: %w", err)
	}

	switch endpoint.Scheme {
	case "http", "https":
		return j.loadRemote(ctx)
	default:
		return j.loadLocal(endpoint.Path)
	}
}

// loadLocal will attempt to load the file containing the JWK Set from the local disk.
func (j *JWKS) loadLocal(path string) (*LoadedJWKS, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read local JWKS file: %w", err)
	}

	kFunc, err := keyfunc.NewJWKSetJSON(contents)
	if err != nil {
		return nil, fmt.Errorf("failed to create key func from file contents: %w", err)
	}

	return &LoadedJWKS{
		KeyFunc: kFunc.Keyfunc,
	}, nil
}

// loadRemote will load the contents of the JWK Set from a remote endpoint.
// It will also ensure that the endpoint is scanned from time to time (`CacheTTL`)
// to ensure new tokens are visible.
func (j *JWKS) loadRemote(ctx context.Context) (*LoadedJWKS, error) {
	kFunc, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{j.Endpoint}, keyfunc.Override{
		RefreshInterval: j.CacheTTL,
		HTTPTimeout:     j.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create key func from remote endpoint: %w", err)
	}

	return &LoadedJWKS{
		KeyFunc: kFunc.Keyfunc,
	}, nil
}

func (j *JWKS) RegisterFlags(fs *pflag.FlagSet, prefix string) {
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
