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

// RBACMiddleware checks that the authenticated request carries the required
// role. Roles are read from the request context, where JWTMiddleware places
// them after validating the token.
//
// SECURITY NOTE: previous versions read the role from the client-supplied
// `X-User-Role` header, which any caller could forge to escalate privileges.
// That path has been removed: authorization decisions must never depend on
// unauthenticated input. If you need role checks without JWT, set the roles
// in the context yourself (e.g. after mTLS/service-token verification).
func RBACMiddleware(required Role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles, ok := GetRoles(r.Context())
		if !ok {
			http.Error(w, "forbidden: insufficient role", http.StatusForbidden)
			return
		}
		for _, role := range roles {
			if strings.EqualFold(role, string(required)) {
				next.ServeHTTP(w, r)
				return
			}
		}
		http.Error(w, "forbidden: insufficient role", http.StatusForbidden)
	})
}
