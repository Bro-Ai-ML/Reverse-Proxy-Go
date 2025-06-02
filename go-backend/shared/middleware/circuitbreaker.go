package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/sony/gobreaker"
)

// CircuitBreakerMiddleware wraps an HTTP handler with circuit breaker functionality.
// It returns a 503 Service Unavailable response when the circuit is open.
func CircuitBreakerMiddleware(cb *gobreaker.CircuitBreaker, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a response recorder to capture the status code
		rec := httptest.NewRecorder()
		
		// Execute the handler through the circuit breaker
		_, err := cb.Execute(func() (interface{}, error) {
			next.ServeHTTP(rec, r)
			
			// If the status code is 5xx, return an error to trip the circuit
			if rec.Code >= http.StatusInternalServerError {
				return nil, fmt.Errorf("server error: %d", rec.Code)
			}
			return nil, nil
		})

		// If the circuit is open, return 503
		if err == gobreaker.ErrOpenState {
			http.Error(w, "service unavailable (circuit open)", http.StatusServiceUnavailable)
			return
		}

		// Copy the recorded response to the original writer
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	})
}
