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
	// to register. If nil all endpoints are allowed.
	Endpoints []string
}

type Verifier interface {
	VerifyEndpointToken(token string) (EndpointToken, error)
}
