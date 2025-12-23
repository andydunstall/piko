package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/dragonflydb/piko/pkg/auth"
	"github.com/dragonflydb/piko/pkg/log"
)

type fakeVerifier struct {
	handler func(token string) (*auth.Token, error)
}

func (v *fakeVerifier) Verify(token string) (*auth.Token, error) {
	return v.handler(token)
}

var _ auth.Verifier = &fakeVerifier{}

type errorMessage struct {
	Error string `json:"error"`
}

func TestAuth(t *testing.T) {
	t.Run("authorization ok", func(t *testing.T) {
		verifier := auth.NewMultiTenantVerifier(&fakeVerifier{
			handler: func(token string) (*auth.Token, error) {
				assert.Equal(t, "123", token)
				return &auth.Token{
					Expiry:    time.Now().Add(time.Hour),
					Endpoints: []string{"e1", "e2", "e3"},
				}, nil
			},
		}, nil)
		m := NewAuth(verifier, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Bearer 123")

		m.Verify(c)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the token was added to context.
		token, ok := c.Get(TokenContextKey)
		assert.True(t, ok)
		assert.Equal(t, []string{"e1", "e2", "e3"}, token.(*auth.Token).Endpoints)
	})

	t.Run("x-piko-authorization ok", func(t *testing.T) {
		verifier := auth.NewMultiTenantVerifier(&fakeVerifier{
			handler: func(token string) (*auth.Token, error) {
				assert.Equal(t, "123", token)
				return &auth.Token{
					Expiry:    time.Now().Add(time.Hour),
					Endpoints: []string{"e1", "e2", "e3"},
				}, nil
			},
		}, nil)
		m := NewAuth(verifier, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		// Add both x-piko-authorization and Authorization, where
		// x-piko-authorization should take precedence.
		c.Request.Header.Add("x-piko-authorization", "Bearer 123")
		c.Request.Header.Add("Authorization", "Bearer xyz")

		m.Verify(c)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the token was added to context.
		token, ok := c.Get(TokenContextKey)
		assert.True(t, ok)
		assert.Equal(t, []string{"e1", "e2", "e3"}, token.(*auth.Token).Endpoints)
	})

	t.Run("tenant ok", func(t *testing.T) {
		verifier := auth.NewMultiTenantVerifier(&fakeVerifier{
			handler: func(_ string) (*auth.Token, error) {
				t.Error("expected tenant")
				return nil, fmt.Errorf("expected tenant")
			},
		}, map[string]auth.Verifier{
			"tenant-1": &fakeVerifier{
				handler: func(token string) (*auth.Token, error) {
					assert.Equal(t, "123", token)
					return &auth.Token{
						Expiry:    time.Now().Add(time.Hour),
						Endpoints: []string{"e1", "e2", "e3"},
					}, nil
				},
			},
		})
		m := NewAuth(verifier, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Bearer 123")
		c.Request.Header.Add("x-piko-tenant-id", "tenant-1")

		m.Verify(c)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the token was added to context.
		token, ok := c.Get(TokenContextKey)
		assert.True(t, ok)
		assert.Equal(t, []string{"e1", "e2", "e3"}, token.(*auth.Token).Endpoints)
	})

	t.Run("invalid token", func(t *testing.T) {
		verifier := auth.NewMultiTenantVerifier(&fakeVerifier{
			handler: func(token string) (*auth.Token, error) {
				assert.Equal(t, "123", token)
				return &auth.Token{}, fmt.Errorf("foo: %w", auth.ErrInvalidToken)
			},
		}, nil)
		m := NewAuth(verifier, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Bearer 123")

		m.Verify(c)

		resp := w.Result()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errMessage errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&errMessage))
		assert.Equal(t, "invalid token", errMessage.Error)
	})

	t.Run("expired token", func(t *testing.T) {
		verifier := auth.NewMultiTenantVerifier(&fakeVerifier{
			handler: func(token string) (*auth.Token, error) {
				assert.Equal(t, "123", token)
				return &auth.Token{}, fmt.Errorf("foo: %w", auth.ErrExpiredToken)
			},
		}, nil)
		m := NewAuth(verifier, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Bearer 123")

		m.Verify(c)

		resp := w.Result()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errMessage errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&errMessage))
		assert.Equal(t, "expired token", errMessage.Error)
	})

	t.Run("unknown tenant", func(t *testing.T) {
		verifier := auth.NewMultiTenantVerifier(&fakeVerifier{
			handler: func(token string) (*auth.Token, error) {
				assert.Equal(t, "123", token)
				return &auth.Token{}, fmt.Errorf("foo: %w", auth.ErrExpiredToken)
			},
		}, nil)
		m := NewAuth(verifier, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Bearer 123")
		c.Request.Header.Add("x-piko-tenant-id", "tenant-123")

		m.Verify(c)

		resp := w.Result()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errMessage errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&errMessage))
		assert.Equal(t, "unknown tenant", errMessage.Error)
	})

	t.Run("unknown error", func(t *testing.T) {
		verifier := auth.NewMultiTenantVerifier(&fakeVerifier{
			handler: func(token string) (*auth.Token, error) {
				assert.Equal(t, "123", token)
				return &auth.Token{}, fmt.Errorf("unknown")
			},
		}, nil)
		m := NewAuth(verifier, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Bearer 123")

		m.Verify(c)

		resp := w.Result()
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("unsupported auth type", func(t *testing.T) {
		m := NewAuth(nil, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)
		c.Request.Header.Add("Authorization", "Basic RpY2F0aW9uLgo=")

		m.Verify(c)

		resp := w.Result()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errMessage errorMessage
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&errMessage))
		assert.Equal(t, "unsupported auth type", errMessage.Error)
	})

	t.Run("missing authorization header", func(t *testing.T) {
		m := NewAuth(nil, log.NewNopLogger())

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "http://example.com/foo", nil)

		m.Verify(c)

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
