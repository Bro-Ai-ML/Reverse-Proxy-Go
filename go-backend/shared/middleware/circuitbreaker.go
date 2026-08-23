package middleware

import (
	"fmt"
	"net/http"

	"github.com/sony/gobreaker"
)

// statusRecorder captures the status code written by the wrapped handler
// while streaming headers/body straight to the real ResponseWriter.
//
// NOTE: the previous implementation buffered the whole response through
// httptest.NewRecorder (a testing helper) — it doubled memory usage, broke
// streaming responses and gave attackers a memory-amplification vector.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}
	return s.ResponseWriter.Write(b)
}

// Flush propagates to the underlying writer when supported (SSE/streaming).
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// CircuitBreakerMiddleware wraps an HTTP handler with circuit breaker
// functionality. It returns a 503 Service Unavailable response when the
// circuit is open. 5xx responses are still streamed to the caller untouched
// while being counted as failures by the breaker.
func CircuitBreakerMiddleware(cb *gobreaker.CircuitBreaker, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		_, err := cb.Execute(func() (interface{}, error) {
			next.ServeHTTP(rec, r)

			// If the status code is 5xx, return an error to trip the circuit.
			if rec.status >= http.StatusInternalServerError {
				return nil, fmt.Errorf("server error: %d", rec.status)
			}
			return nil, nil
		})

		// If the circuit rejected the call (open or too many concurrent
		// half-open probes), nothing was written yet: answer 503.
		if err == gobreaker.ErrOpenState || err == gobreaker.ErrTooManyRequests {
			http.Error(w, "service unavailable (circuit open)", http.StatusServiceUnavailable)
			return
		}
		// Any other outcome (success, or a 5xx already streamed to the client)
		// needs no extra handling here.
	})
}
