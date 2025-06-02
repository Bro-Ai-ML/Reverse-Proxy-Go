package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/time/rate"
)

var rateLimitHits = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "ratelimit_hits_total",
		Help: "Rate limit hits",
	},
	[]string{"type", "status"},
)

// RateLimiterConfig holds the configuration for the rate limiting middleware.
// It allows specifying global limits and IP-based limits.
// MaxIPLimiters defines the maximum number of distinct IP addresses to store for rate limiting.
// If this limit is exceeded, the oldest entries might be cleaned up (basic LRU-like behavior implemented).
type RateLimiterConfig struct {
	GlobalLimit   rate.Limit
	GlobalBurst   int
	IPLimit       rate.Limit
	IPBurst       int
	MaxIPLimiters int
	Logger        *slog.Logger // Optional logger, defaults to slog.Default()
}

// RateLimiter provides HTTP middleware for rate limiting requests.
// It supports both a global rate limit and per-IP rate limits.
type RateLimiter struct {
	config        *RateLimiterConfig
	globalLimiter *rate.Limiter
	ipLimiters    map[string]*rate.Limiter
	mu            sync.Mutex
	logger        *slog.Logger
}

// NewRateLimiter creates a new RateLimiter middleware with the given configuration.
// The cfg parameter must not be nil.
func NewRateLimiter(cfg *RateLimiterConfig) *RateLimiter {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &RateLimiter{
		config:        cfg,
		globalLimiter: rate.NewLimiter(cfg.GlobalLimit, cfg.GlobalBurst),
		ipLimiters:    make(map[string]*rate.Limiter),
		logger:        logger,
	}
}

// Middleware returns an http.Handler that applies rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)

		// Check global rate limit first
		if !rl.globalLimiter.Allow() {
			rateLimitHits.WithLabelValues("global", "denied").Inc()
			rl.logger.WarnContext(r.Context(), "Global rate limit exceeded", "ip", ip, "path", r.URL.Path)
			http.Error(w, "Global rate limit exceeded", http.StatusTooManyRequests)

			return
		} else {
			rateLimitHits.WithLabelValues("global", "allowed").Inc()
		}

		// Check IP-based rate limit
		rl.mu.Lock()
		limiter, exists := rl.ipLimiters[ip]

		if !exists {
			limiter = rate.NewLimiter(rl.config.IPLimit, rl.config.IPBurst)
			rl.ipLimiters[ip] = limiter
			// Basic cleanup if map grows too large: remove one old entry (not necessarily the oldest)
			if len(rl.ipLimiters) > rl.config.MaxIPLimiters {
				for k := range rl.ipLimiters {
					if k != ip { // Don't delete the just-added entry
						delete(rl.ipLimiters, k)
						rl.logger.Debug("Cleaned up old IP limiter entry", "ip", k)

						break // Remove one and exit cleanup
					}
				}
			}
		}
		rl.mu.Unlock()

		if !limiter.Allow() {
			rateLimitHits.WithLabelValues("ip", "denied").Inc()
			rl.logger.WarnContext(r.Context(), "IP-based rate limit exceeded", "ip", ip, "path", r.URL.Path)
			http.Error(w, "IP-based rate limit exceeded", http.StatusTooManyRequests)

			return
		} else {
			rateLimitHits.WithLabelValues("ip", "allowed").Inc()
		}

		next.ServeHTTP(w, r)
	})
}

// getIP extracts the client's IP address from the request.
// It checks X-Forwarded-For header first, then r.RemoteAddr.
// The function handles various IP address formats but assumes the request r is not nil.
func getIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	parts := strings.Split(r.RemoteAddr, ":")
	if len(parts) > 0 {
		return parts[0]
	}

	return r.RemoteAddr // Fallback
}
