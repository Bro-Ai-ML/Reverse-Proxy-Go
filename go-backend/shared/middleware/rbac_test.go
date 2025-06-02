package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRBACMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		requiredRole   Role
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "missing role header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/admin", nil)
				return req
			},
			requiredRole:   RoleAdmin,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "forbidden: insufficient role\n",
		},
		{
			name: "insufficient role",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/admin", nil)
				req.Header.Set("X-User-Role", "user")
				return req
			},
			requiredRole:   RoleAdmin,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "forbidden: insufficient role\n",
		},
		{
			name: "exact role match",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/admin", nil)
				req.Header.Set("X-User-Role", "admin")
				return req
			},
			requiredRole:   RoleAdmin,
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name: "case insensitive role check",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/admin", nil)
				req.Header.Set("X-User-Role", "ADMIN")
				return req
			},
			requiredRole:   RoleAdmin,
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
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
			handler := RBACMiddleware(tt.requiredRole, testHandler)

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
