package upstream

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/auth"
)

const (
	TokenContextKey = "_piko_token"
)

// AuthMiddleware verifies the request token.
type AuthMiddleware struct {
	verifier auth.Verifier
	logger   log.Logger
}

func NewAuthMiddleware(verifier auth.Verifier, logger log.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		verifier: verifier,
		logger:   logger,
	}
}

// VerifyEndpointToken verifies the request endpoint token and adds to the
// context.
//
// If the token is invalid, returns 401 to the client.
func (m *AuthMiddleware) VerifyEndpointToken(c *gin.Context) {
	tokenString, ok := m.parseToken(c)
	if !ok {
		return
	}

	token, err := m.verifier.VerifyEndpointToken(tokenString)
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

		m.logger.Warn(
			"unknown verification error",
			zap.Error(err),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Set(TokenContextKey, &token)
	c.Next()
}

func (m *AuthMiddleware) parseToken(c *gin.Context) (string, bool) {
	authorization := c.Request.Header.Get("Authorization")
	authType, tokenString, ok := strings.Cut(authorization, " ")
	if !ok {
		m.logger.Warn("missing authorization header")
		c.AbortWithStatusJSON(
			http.StatusUnauthorized,
			gin.H{"error": "missing authorization"},
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
