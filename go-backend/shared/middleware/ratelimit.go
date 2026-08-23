package middleware

import (
	"log/slog"
	"net"
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

	// TrustedProxies lists peer IPs (e.g. your load balancer) whose
	// X-Forwarded-For header may be trusted. When set, XFF is honored only
	// for requests arriving from one of these peers, and the client IP is
	// resolved as the right-most untrusted hop (spoof-proof). When empty,
	// the historical behavior is kept (left-most XFF entry is used) — deploy
	// behind a proxy? Set this, otherwise any client can bypass the limiter
	// by forging X-Forwarded-For.
	TrustedProxies []string
}

// RateLimiter provides HTTP middleware for rate limiting requests.
// It supports both a global rate limit and per-IP rate limits.
type RateLimiter struct {
	config         *RateLimiterConfig
	globalLimiter  *rate.Limiter
	ipLimiters     map[string]*rate.Limiter
	trustedProxies map[string]struct{}
	mu             sync.Mutex
	logger         *slog.Logger
}

// NewRateLimiter creates a new RateLimiter middleware with the given configuration.
// The cfg parameter must not be nil.
func NewRateLimiter(cfg *RateLimiterConfig) *RateLimiter {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	trusted := make(map[string]struct{}, len(cfg.TrustedProxies))
	for _, p := range cfg.TrustedProxies {
		trusted[strings.TrimSpace(p)] = struct{}{}
	}

	return &RateLimiter{
		config:         cfg,
		globalLimiter:  rate.NewLimiter(cfg.GlobalLimit, cfg.GlobalBurst),
		ipLimiters:     make(map[string]*rate.Limiter),
		trustedProxies: trusted,
		logger:         logger,
	}
}

// Middleware returns an http.Handler that applies rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.getClientIP(r)

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

// peerIP extracts the direct peer's IP from r.RemoteAddr, handling
// "host:port", IPv6 "[::1]:80" and bare addresses.
func peerIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// getClientIP extracts the client's IP address from the request.
//
// When TrustedProxies is configured, X-Forwarded-For is honored only for
// requests whose direct peer is a trusted proxy, and the client IP is the
// right-most hop in the header that is NOT itself a trusted proxy — this is
// the spoof-proof resolution used by production load-balanced deployments.
//
// When TrustedProxies is empty, the historical behavior is preserved: the
// left-most X-Forwarded-For entry is used. Be aware that in that mode any
// client can forge the header to bypass per-IP limits.
func (rl *RateLimiter) getClientIP(r *http.Request) string {
	peer := peerIP(r)
	forwarded := r.Header.Get("X-Forwarded-For")

	if forwarded != "" && len(rl.trustedProxies) > 0 {
		if _, trusted := rl.trustedProxies[peer]; trusted {
			ips := strings.Split(forwarded, ",")
			// Walk right-to-left: skip trusted proxies, return the first
			// untrusted hop (the real client).
			for i := len(ips) - 1; i >= 0; i-- {
				ip := strings.TrimSpace(ips[i])
				if ip == "" {
					continue
				}
				if _, isProxy := rl.trustedProxies[ip]; !isProxy {
					return ip
				}
			}
			// All hops trusted: fall back to the left-most entry.
			if ip := strings.TrimSpace(ips[0]); ip != "" {
				return ip
			}
		}
		// Untrusted peer: its XFF header is attacker-controlled; use the peer.
		return peer
	}

	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if ip := strings.TrimSpace(ips[0]); ip != "" {
			return ip
		}
	}

	return peer
}
