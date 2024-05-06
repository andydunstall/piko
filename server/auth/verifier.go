package auth

import (
	"errors"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

type EndpointToken struct {
	// Expiry contains the time the token expires, or zero if there is no
	// expiry.
	Expiry time.Time

	// Endpoints contains the list of endpoint IDs the connection is permitted
	// to register. If empty then all endpoints are allowed.
	Endpoints []string
}

// EndpointPermitted returns whether the given endpoint ID is permitted for
// this token.
func (t *EndpointToken) EndpointPermitted(endpointID string) bool {
	if len(t.Endpoints) == 0 {
		// If 'Endpoints' is empty then all endpoints are allowed.
		return true
	}
	for _, id := range t.Endpoints {
		if endpointID == id {
			return true
		}
	}
	return false
}

type Verifier interface {
	VerifyEndpointToken(token string) (EndpointToken, error)
}
