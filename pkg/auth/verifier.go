package auth

import (
	"errors"
	"slices"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

// Token represents an authenticated Piko token.
type Token struct {
	// Expiry contains the time the token expires, or zero if there is no
	// expiry.
	Expiry time.Time

	// Endpoints contains the list of endpoint IDs the connection is permitted
	// to access (either connect to or listen on). If empty then all endpoints
	// are allowed.
	Endpoints []string
}

// EndpointPermitted returns whether the token it permitted to access the
// endpoint with the given ID.
//
// If the token doesn't include any endpoints, it can access all endpoints.
func (t *Token) EndpointPermitted(endpointID string) bool {
	if len(t.Endpoints) == 0 {
		return true
	}
	return slices.Contains(t.Endpoints, endpointID)
}

// Verifier verifies client tokens.
type Verifier interface {
	Verify(token string) (*Token, error)
}
