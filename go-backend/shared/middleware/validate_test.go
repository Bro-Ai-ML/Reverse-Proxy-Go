package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateMiddleware(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler := ValidateMiddleware(testHandler)
	handler.ServeHTTP(rr, req)

	// Currently just tests that the request passes through
	assert.Equal(t, http.StatusOK, rr.Code)
}
