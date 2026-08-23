package middleware

import (
	"mime"
	"net/http"
)

// maxRequestBodyBytes is the default body cap applied by ValidateMiddleware.
// Handlers that need different limits should wrap r.Body with
// http.MaxBytesReader themselves.
const maxRequestBodyBytes = 2 * 1024 * 1024

// ValidateMiddleware applies basic, protocol-level request validation:
//   - requests carrying a body must declare a JSON content type (parameters
//     such as "charset=utf-8" are accepted),
//   - request bodies are hard-capped with http.MaxBytesReader so the limit
//     cannot be bypassed by lying about Content-Length or by using chunked
//     transfer encoding (the previous check trusted r.ContentLength, which a
//     malicious client controls).
func ValidateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasBody := r.ContentLength != 0 && r.Body != nil && r.Body != http.NoBody

		if hasBody {
			contentType := r.Header.Get("Content-Type")
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			mediaType, _, err := mime.ParseMediaType(contentType)
			if err != nil || mediaType != "application/json" {
				http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
				return
			}
			// Enforce the limit at read time instead of trusting the
			// client-provided Content-Length header.
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		}

		next.ServeHTTP(w, r)
	})
}
