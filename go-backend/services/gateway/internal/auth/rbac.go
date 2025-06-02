package auth

import (
	"net/http"
)

func RequireRole(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roles, ok := r.Context().Value(RolesKey).([]string)
			if !ok {
				http.Error(w, "No roles found in context", http.StatusForbidden)
				return
			}
			for _, role := range roles {
				if role == requiredRole {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "Insufficient permissions", http.StatusForbidden)
		})
	}
}

func RequireAnyRole(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roles, ok := r.Context().Value(RolesKey).([]string)
			if !ok {
				http.Error(w, "No roles found in context", http.StatusForbidden)
				return
			}
			for _, requiredRole := range requiredRoles {
				for _, role := range roles {
					if role == requiredRole {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			http.Error(w, "Insufficient permissions", http.StatusForbidden)
		})
	}
}
