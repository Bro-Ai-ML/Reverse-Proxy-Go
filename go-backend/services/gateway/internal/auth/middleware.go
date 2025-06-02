package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const (
	UserIDKey contextKey = "user_id"
	RolesKey  contextKey = "roles"
)

// AuthMiddleware devient une méthode du type *JWTManager
func (j *JWTManager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicRoute(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}
		claims, err := j.ValidateToken(parts[1])
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, RolesKey, claims.Roles)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isPublicRoute(path string) bool {
	publicRoutes := []string{
		"/health",
		"/auth/login",
		"/auth/refresh",
	}
	for _, route := range publicRoutes {
		if path == route {
			return true
		}
	}
	return false
}
