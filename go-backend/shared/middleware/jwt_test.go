package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestJWTMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedStatus int
		expectedBody   string
		setupJWT       func() string
	}{
		{
			name: "missing authorization header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "missing or invalid Authorization header\n",
		},
		{
			name: "invalid token format",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "InvalidToken")
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "missing or invalid Authorization header\n",
		},
		{
			name: "invalid token signature",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer invalid.token.here")
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "invalid token\n",
		},
		{
			name: "valid token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				token := generateTestToken(t, "test-user", []string{"user"}, time.Now().Add(1*time.Hour))
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "expired token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				token := generateTestToken(t, "test-user", []string{"user"}, time.Now().Add(-1*time.Hour))
				req.Header.Set("Authorization", "Bearer "+token)
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "invalid token\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that will be wrapped by the middleware
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			// Create the middleware chain
			handler := JWTMiddleware(testHandler)

			// Create a response recorder
			w := httptest.NewRecorder()

			// Create a request
			req := tt.setupRequest()

			// Serve the request
			handler.ServeHTTP(w, req)


			// Verify the response
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

// generateTestToken generates a JWT token for testing
func generateTestToken(t *testing.T, userID string, roles []string, expiresAt time.Time) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"roles":   roles,
		"exp":      expiresAt.Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}
	return tokenString
}
