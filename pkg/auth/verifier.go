package auth

import (
	"errors"
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

// Verifier verifies client tokens.
type Verifier interface {
	Verify(token string) (*Token, error)
}
