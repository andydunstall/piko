package upstream

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/auth"
)

type fakeVerifier struct {
	handler func(token string) (auth.EndpointToken, error)
}

func (v *fakeVerifier) VerifyEndpointToken(token string) (auth.EndpointToken, error) {
	return v.handler(token)
}

var _ auth.Verifier = &fakeVerifier{}

type errorMessage struct {
	Error string `json:"error"`
}

func TestAuthMiddleware(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		verifier := &fakeVerifier{
			handler: func(token string) (auth.EndpointToken, error) {
				assert.Equal(t, "123", token)
				return auth.EndpointToken{
					Expiry:    time.Now().Add(time.Hour),
					Endpoints: []string{"e1", "e2", "e3"},
				}, nil
			},
		}
		m := NewAuthMiddleware(verifier, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Bearer 123")

		m.VerifyEndpointToken(c)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the token was added to context.
		token, ok := c.Get(TokenContextKey)
		assert.True(t, ok)
		assert.Equal(t, []string{"e1", "e2", "e3"}, token.(*auth.EndpointToken).Endpoints)
	})

	t.Run("invalid token", func(t *testing.T) {
		verifier := &fakeVerifier{
			handler: func(token string) (auth.EndpointToken, error) {
				assert.Equal(t, "123", token)
				return auth.EndpointToken{}, fmt.Errorf("foo: %w", auth.ErrInvalidToken)
			},
		}
		m := NewAuthMiddleware(verifier, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Bearer 123")

		m.VerifyEndpointToken(c)

		resp := w.Result()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errMessage errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&errMessage))
		assert.Equal(t, "invalid token", errMessage.Error)
	})

	t.Run("expired token", func(t *testing.T) {
		verifier := &fakeVerifier{
			handler: func(token string) (auth.EndpointToken, error) {
				assert.Equal(t, "123", token)
				return auth.EndpointToken{}, fmt.Errorf("foo: %w", auth.ErrExpiredToken)
			},
		}
		m := NewAuthMiddleware(verifier, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Bearer 123")

		m.VerifyEndpointToken(c)

		resp := w.Result()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errMessage errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&errMessage))
		assert.Equal(t, "expired token", errMessage.Error)
	})

	t.Run("unknown error", func(t *testing.T) {
		verifier := &fakeVerifier{
			handler: func(token string) (auth.EndpointToken, error) {
				assert.Equal(t, "123", token)
				return auth.EndpointToken{}, fmt.Errorf("unknown")
			},
		}
		m := NewAuthMiddleware(verifier, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Bearer 123")

		m.VerifyEndpointToken(c)

		resp := w.Result()
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("unsupported auth type", func(t *testing.T) {
		m := NewAuthMiddleware(nil, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Basic RpY2F0aW9uLgo=")

		m.VerifyEndpointToken(c)

		resp := w.Result()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errMessage errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&errMessage))
		assert.Equal(t, "unsupported auth type", errMessage.Error)
	})

	t.Run("missing authorization header", func(t *testing.T) {
		m := NewAuthMiddleware(nil, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)

		m.VerifyEndpointToken(c)

		resp := w.Result()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errMessage errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&errMessage))
		assert.Equal(t, "missing authorization", errMessage.Error)
	})
}

func init() {
	// Disable Gin debug logs.
	gin.SetMode(gin.ReleaseMode)
}
