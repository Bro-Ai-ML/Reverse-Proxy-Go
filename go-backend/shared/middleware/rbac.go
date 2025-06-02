package middleware

import (
	"net/http"
	"strings"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleUser   Role = "user"
	RoleViewer Role = "viewer"
)

// RBACMiddleware checks if the user has the required role.
func RBACMiddleware(required Role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := r.Header.Get("X-User-Role")
		if !strings.EqualFold(role, string(required)) {
			http.Error(w, "forbidden: insufficient role", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
