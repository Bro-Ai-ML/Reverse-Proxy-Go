package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"
)

// Context keys used to propagate authenticated identity to downstream handlers.
type contextKey string

const (
	// UserIDContextKey holds the authenticated user id (string) when available.
	UserIDContextKey contextKey = "auth_user_id"
	// RolesContextKey holds the authenticated user's roles ([]string) when available.
	RolesContextKey contextKey = "auth_roles"
)

var (
	jwtSecretOnce sync.Once
	// jwtSecret holds the HMAC signing key. It is loaded lazily from JWT_SECRET
	// on first use instead of at package init, so importing this package can no
	// longer kill the process (previous behavior: log.Fatal in init).
	jwtSecret []byte
	// jwtSecretMissing is true when JWT_SECRET was not configured. The
	// middleware then fails closed with 503 instead of accepting tokens signed
	// with an empty key.
	jwtSecretMissing bool
)

// loadJWTSecret loads the shared secret from the environment exactly once.
func loadJWTSecret() {
	jwtSecretOnce.Do(func() {
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			jwtSecretMissing = true
			return
		}
		jwtSecret = []byte(secret)
	})
}

// GetUserID returns the authenticated user id placed in the context by
// JWTMiddleware, if any.
func GetUserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(UserIDContextKey).(string)
	return v, ok
}

// GetRoles returns the authenticated roles placed in the context by
// JWTMiddleware, if any.
func GetRoles(ctx context.Context) ([]string, bool) {
	v, ok := ctx.Value(RolesContextKey).([]string)
	return v, ok
}

func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loadJWTSecret()
		if jwtSecretMissing {
			// Fail closed: never validate tokens without a configured secret.
			http.Error(w, "authentication is not configured", http.StatusServiceUnavailable)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if tokenStr == "" {
			http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		}, jwt.WithValidMethods([]string{"HS256"}))

		if err != nil || token == nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		// Propagate identity to downstream handlers so authentication is not
		// silently discarded (previous behavior validated then forgot who
		// called).
		ctx := r.Context()
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if uid, ok := claims["user_id"].(string); ok {
				ctx = context.WithValue(ctx, UserIDContextKey, uid)
			}
			if rawRoles, ok := claims["roles"].([]interface{}); ok {
				roles := make([]string, 0, len(rawRoles))
				for _, rr := range rawRoles {
					if s, ok := rr.(string); ok {
						roles = append(roles, s)
					}
				}
				ctx = context.WithValue(ctx, RolesContextKey, roles)
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
