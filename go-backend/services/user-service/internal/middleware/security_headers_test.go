package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	h := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	headers := []string{
		"Content-Security-Policy",
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Permissions-Policy",
		"Strict-Transport-Security",
	}
	for _, hname := range headers {
		if rw.Header().Get(hname) == "" {
			t.Errorf("header %s missing", hname)
		}
	}
}
