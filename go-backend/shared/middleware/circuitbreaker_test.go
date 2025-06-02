package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
)

func TestCircuitBreakerMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		setupHandler   func() http.Handler
		expectedStatus int
		expectedBody   string
		shouldOpen     bool
	}{
		{
			name: "successful request",
			setupHandler: func() http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("success"))
				})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "success",
			shouldOpen:     false,
		},
		{
			name: "failing request",
			setupHandler: func() http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "internal server error", http.StatusInternalServerError)
				})
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal server error\n",
			shouldOpen:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a circuit breaker with aggressive settings for testing
			cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
				Name:        "test",
				MaxRequests: 1,
				Interval:    0, // No interval for testing
				Timeout:     0, // No timeout for testing
				ReadyToTrip: func(counts gobreaker.Counts) bool {
					return counts.ConsecutiveFailures > 0
				},
			})


			// Create middleware with test handler
			handler := CircuitBreakerMiddleware(cb, tt.setupHandler())

			// Create test request
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

			// Execute request
			handler.ServeHTTP(w, req)


			// Verify response
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())

			// Verify circuit state
			if tt.shouldOpen {
				assert.Equal(t, gobreaker.StateOpen, cb.State())
			} else {
				assert.Equal(t, gobreaker.StateClosed, cb.State())
			}
		})
	}
}

func TestCircuitBreakerOpensAfterFailures(t *testing.T) {
	// Create a circuit breaker that opens after 1 failure
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "test",
		MaxRequests: 1,
		Interval:    0, // No interval for testing
		Timeout:     0, // No timeout for testing
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 0
		},
	})

	// Create a failing handler
	handler := CircuitBreakerMiddleware(cb, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))

	req := httptest.NewRequest("GET", "/test", nil)

	// First request should fail and trip the circuit
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req)
	assert.Equal(t, http.StatusInternalServerError, w1.Code)
	
	// The circuit should now be open due to the 5xx error
	assert.Equal(t, gobreaker.StateOpen, cb.State())

	// Second request should be rejected with 503
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusServiceUnavailable, w2.Code)
	assert.Equal(t, "service unavailable (circuit open)\n", w2.Body.String())
	assert.Equal(t, gobreaker.StateOpen, cb.State())
}
