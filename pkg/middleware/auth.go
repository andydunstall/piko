package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dragonflydb/piko/pkg/auth"
	"github.com/dragonflydb/piko/pkg/log"
)

const (
	TokenContextKey = "_piko_token"
)

// Auth is middleware to verify token requests.
type Auth struct {
	verifier *auth.MultiTenantVerifier
	logger   log.Logger
}

func NewAuth(verifier *auth.MultiTenantVerifier, logger log.Logger) *Auth {
	return &Auth{
		verifier: verifier,
		logger:   logger,
	}
}

// Verify verifies the request endpoint token and adds to the context.
//
// If the token is invalid, returns 401 to the client.
func (m *Auth) Verify(c *gin.Context) {
	tokenString, ok := m.parseToken(c)
	if !ok {
		return
	}

	tenantID := m.parseTenant(c)

	token, err := m.verifier.Verify(tokenString, tenantID)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidToken) {
			m.logger.Warn(
				"auth invalid token",
				zap.Error(err),
			)
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{"error": "invalid token"},
			)
			return
		}
		if errors.Is(err, auth.ErrExpiredToken) {
			m.logger.Warn(
				"auth expired token",
				zap.Error(err),
			)
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{"error": "expired token"},
			)
			return
		}
		if errors.Is(err, auth.ErrUnknownTenant) {
			m.logger.Warn(
				"auth unknwon tenant",
				zap.Error(err),
			)
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{"error": "unknown tenant"},
			)
			return
		}

		m.logger.Warn(
			"unknown verification error",
			zap.Error(err),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Set(TokenContextKey, token)
	c.Next()
}

func (m *Auth) parseToken(c *gin.Context) (string, bool) {
	// Support both x-piko-authorization and authorization, where
	// x-piko-authorization takes precedence. x-piko-authorization can be used
	// to avoid conflicts with the upstream authorization header.
	authorization := c.Request.Header.Get("x-piko-authorization")
	if authorization == "" {
		authorization = c.Request.Header.Get("Authorization")
	}
	if authorization == "" {
		m.logger.Warn("missing authorization header")
		c.AbortWithStatusJSON(
			http.StatusUnauthorized,
			gin.H{"error": "missing authorization"},
		)
		return "", false
	}
	authType, tokenString, ok := strings.Cut(authorization, " ")
	if !ok {
		m.logger.Warn("invalid authorization header")
		c.AbortWithStatusJSON(
			http.StatusUnauthorized,
			gin.H{"error": "invalid authorization"},
		)
		return "", false
	}
	if authType != "Bearer" {
		m.logger.Warn(
			"unsupported auth type",
			zap.String("auth-type", authType),
		)
		c.AbortWithStatusJSON(
			http.StatusUnauthorized,
			gin.H{"error": "unsupported auth type"},
		)
		return "", false
	}

	return tokenString, true
}

func (m *Auth) parseTenant(c *gin.Context) string {
	return c.Request.Header.Get("x-piko-tenant-id")
}
