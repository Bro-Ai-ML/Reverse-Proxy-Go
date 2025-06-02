package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	targetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := SecurityHeadersMiddleware(targetHandler)

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	resp := rr.Result()
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing security header")
	}
}
