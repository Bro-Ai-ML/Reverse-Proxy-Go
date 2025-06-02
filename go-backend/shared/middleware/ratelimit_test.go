package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

type testLogger struct {
	msgs []string
}

func (l *testLogger) Handler() slog.Handler {
	return l
}

func (l *testLogger) Enabled(context.Context, slog.Level) bool { return true }
func (l *testLogger) WithGroup(name string) slog.Handler { return l }
func (l *testLogger) WithAttrs(attrs []slog.Attr) slog.Handler { return l }

func (l *testLogger) Handle(ctx context.Context, r slog.Record) error {
	l.msgs = append(l.msgs, r.Message)
	return nil
}

func TestRateLimiter(t *testing.T) {
	testLogger := &testLogger{}
	logger := slog.New(testLogger)
	
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		setupLimiter   func() *RateLimiter
		expectedStatus int
		expectedBody   string
		expectedLogs   []string
	}{
		{
			name: "global rate limit",
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			setupLimiter: func() *RateLimiter {
				return NewRateLimiter(&RateLimiterConfig{
					GlobalLimit:   1,
					GlobalBurst:   1,
					IPLimit:       10,
					IPBurst:       10,
					MaxIPLimiters: 100,
					Logger:        logger,
				})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
			expectedLogs:   []string{},
		},
		{
			name: "global rate limit exceeded",
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/test", nil)
			},
			setupLimiter: func() *RateLimiter {
				rl := NewRateLimiter(&RateLimiterConfig{
					GlobalLimit:   1,
					GlobalBurst:   1,
					IPLimit:       10,
					IPBurst:       10,
					MaxIPLimiters: 100,
					Logger:        logger,
				})
				// Consume the burst
				rl.globalLimiter.AllowN(time.Now(), 1)
				return rl
			},
			expectedStatus: http.StatusTooManyRequests,
			expectedBody:   "Global rate limit exceeded\n",
			expectedLogs:   []string{"Global rate limit exceeded"},
		},
		{
			name: "IP rate limit",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.1:12345"
				return req
			},
			setupLimiter: func() *RateLimiter {
				return NewRateLimiter(&RateLimiterConfig{
					GlobalLimit:   100,
					GlobalBurst:   100,
					IPLimit:       1,
					IPBurst:       1,
					MaxIPLimiters: 100,
					Logger:        logger,
				})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
			expectedLogs:   []string{},
		},
		{
			name: "IP rate limit exceeded",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.2:12345"
				return req
			},
			setupLimiter: func() *RateLimiter {
				rl := NewRateLimiter(&RateLimiterConfig{
					GlobalLimit:   100,
					GlobalBurst:   100,
					IPLimit:       1,
					IPBurst:       1,
					MaxIPLimiters: 100,
					Logger:        logger,
				})
				// Consume the burst for this IP
				ip := "192.168.1.2"
				limiter := rate.NewLimiter(1, 1)
				limiter.AllowN(time.Now(), 1)
				rl.mu.Lock()
				rl.ipLimiters[ip] = limiter
				rl.mu.Unlock()
				return rl
			},
			expectedStatus: http.StatusTooManyRequests,
			expectedBody:   "IP-based rate limit exceeded\n",
			expectedLogs:   []string{"IP-based rate limit exceeded"},
		},
		{
			name: "IP from X-Forwarded-For",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")
				req.RemoteAddr = "192.168.1.1:12345"
				return req
			},
			setupLimiter: func() *RateLimiter {
				return NewRateLimiter(&RateLimiterConfig{
					GlobalLimit:   100,
					GlobalBurst:   100,
					IPLimit:       1,
					IPBurst:       1,
					MaxIPLimiters: 100,
					Logger:        logger,
				})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
			expectedLogs:   []string{},
		},
		{
			name: "cleanup old IP limiters",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.100:12345"
				return req
			},
			setupLimiter: func() *RateLimiter {
				rl := NewRateLimiter(&RateLimiterConfig{
					GlobalLimit:   100,
					GlobalBurst:   100,
					IPLimit:       1,
					IPBurst:       1,
					MaxIPLimiters: 2,
					Logger:        logger,
				})
				// Add two IPs to the limiters map
				rl.mu.Lock()
				rl.ipLimiters["192.168.1.1"] = rate.NewLimiter(1, 1)
				rl.ipLimiters["192.168.1.2"] = rate.NewLimiter(1, 1)
				rl.mu.Unlock()
				return rl
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
			expectedLogs:   []string{"Cleaned up old IP limiter entry"},
		},
		{
			name: "getIP fallback to raw RemoteAddr",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "unexpected-addr-format"
				return req
			},
			setupLimiter: func() *RateLimiter {
				return NewRateLimiter(&RateLimiterConfig{
					GlobalLimit:   1,
					GlobalBurst:   1,
					IPLimit:       1,
					IPBurst:       1,
					MaxIPLimiters: 100,
					Logger:        logger,
				})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
			expectedLogs:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset test logger
			testLogger.msgs = nil

			// Create a test handler that will be wrapped by the middleware
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			// Create the rate limiter
			rateLimiter := tt.setupLimiter()

			// Create the middleware chain
			handler := rateLimiter.Middleware(testHandler)

			// Create a response recorder
			w := httptest.NewRecorder()

			// Create a request
			req := tt.setupRequest()

			// Serve the request
			handler.ServeHTTP(w, req)


			// Verify the response
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())

			// Verify the logs
			if len(tt.expectedLogs) > 0 {
				assert.GreaterOrEqual(t, len(testLogger.msgs), len(tt.expectedLogs))
				for _, expectedLog := range tt.expectedLogs {
					found := false
					for _, msg := range testLogger.msgs {
						if msg == expectedLog {
							found = true
							break
						}
					}
					assert.True(t, found, "expected log message not found: %s", expectedLog)
				}
			} else {
				assert.Empty(t, testLogger.msgs)
			}
		})
	}
}
