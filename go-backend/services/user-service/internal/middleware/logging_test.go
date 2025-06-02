package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

func TestLoggingMiddleware(t *testing.T) {
	logger := zerolog.Nop()
	h := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rw.Code)
	}
	if rw.Body.String() != "ok" {
		t.Errorf("unexpected body: %s", rw.Body.String())
	}
}
