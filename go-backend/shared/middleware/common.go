package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// SecurityHeadersMiddleware adds common security-related HTTP headers.
// It sets X-Content-Type-Options to nosniff, X-Frame-Options to DENY,
// and X-XSS-Protection to 1; mode=block.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs HTTP requests with method, path, and duration.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		slog.Info("request completed", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

// SecureHeadersMiddleware ajoute des headers de sécurité essentiels (CSP, X-Content-Type-Options, etc.)
func SecureHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		// Ajoute d'autres headers si besoin
		next.ServeHTTP(w, r)
	})
}
