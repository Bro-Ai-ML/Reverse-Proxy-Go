package middleware

import (
	"net/http"
)

func ValidateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Add real input validation logic here
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
		if r.ContentLength > 2*1024*1024 {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		next.ServeHTTP(w, r)
	})
}
