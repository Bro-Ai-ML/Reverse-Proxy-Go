package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// withRoles returns a request whose context carries the given roles, as
// JWTMiddleware would set them after validating a token.
func withRoles(roles []string) *http.Request {
	req := httptest.NewRequest("GET", "/admin", nil)
	if roles != nil {
		ctx := context.WithValue(req.Context(), RolesContextKey, roles)
		return req.WithContext(ctx)
	}
	return req
}

func TestRBACMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		requiredRole   Role
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "missing roles in context",
			setupRequest:   func() *http.Request { return withRoles(nil) },
			requiredRole:   RoleAdmin,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "forbidden: insufficient role\n",
		},
		{
			name:           "insufficient role",
			setupRequest:   func() *http.Request { return withRoles([]string{"user"}) },
			requiredRole:   RoleAdmin,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "forbidden: insufficient role\n",
		},
		{
			name:           "exact role match",
			setupRequest:   func() *http.Request { return withRoles([]string{"admin"}) },
			requiredRole:   RoleAdmin,
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "case insensitive role check",
			setupRequest:   func() *http.Request { return withRoles([]string{"ADMIN"}) },
			requiredRole:   RoleAdmin,
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "forged client header is ignored",
			setupRequest: func() *http.Request {
				req := withRoles(nil)
				// Clients used to be able to escalate by sending this header;
				// it must no longer grant anything.
				req.Header.Set("X-User-Role", "admin")
				return req
			},
			requiredRole:   RoleAdmin,
			expectedStatus: http.StatusForbidden,
			expectedBody:   "forbidden: insufficient role\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			})

			handler := RBACMiddleware(tt.requiredRole, testHandler)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, tt.setupRequest())

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}
