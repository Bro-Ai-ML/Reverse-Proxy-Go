package config

import (
	"errors"
	"time"

	"golang.org/x/time/rate"
)

type ServiceConfig struct {
	MaxUsageQueueSize    int           `json:"max_usage_queue_size"`
	MaxAlertQueueSize    int           `json:"max_alert_queue_size"`
	WorkerPoolSize       int           `json:"worker_pool_size"`
	AlertWorkerPoolSize  int           `json:"alert_worker_pool_size"`
	ShutdownTimeout      time.Duration `json:"shutdown_timeout"`
	RequestTimeout       time.Duration `json:"request_timeout"`
	EnableCircuitBreaker bool          `json:"enable_circuit_breaker"`
	Port                 string        `json:"port"`
	// Fields for shared RateLimiterConfig will be set in main.go directly
	// IPRateLimit        rate.Limit `json:"ip_rate_limit"`       // Removed
	// IPRateBurst        int        `json:"ip_rate_burst"`        // Removed
	// MaxIPRateLimiters  int        `json:"max_ip_rate_limiters"` // Removed
	MaxRequestSizeMB int64 `json:"max_request_size_mb"`

	// New fields for shared rate limiter configuration
	GlobalRateLimit   rate.Limit `json:"global_rate_limit"`
	GlobalRateBurst   int        `json:"global_rate_burst"`
	IPRateLimit       rate.Limit `json:"ip_rate_limit_shared"` // Renamed to avoid conflict if old fields were kept
	IPRateBurst       int        `json:"ip_rate_burst_shared"`
	MaxIPRateLimiters int        `json:"max_ip_rate_limiters_shared"`
}

func (c *ServiceConfig) Validate() error {
	if c.WorkerPoolSize <= 0 {
		return errors.New("WorkerPoolSize must be positive")
	}
	if c.AlertWorkerPoolSize <= 0 {
		return errors.New("AlertWorkerPoolSize must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return errors.New("ShutdownTimeout must be positive")
	}
	if c.RequestTimeout <= 0 {
		return errors.New("RequestTimeout must be positive")
	}
	if c.MaxUsageQueueSize <= 0 {
		return errors.New("MaxUsageQueueSize must be positive")
	}
	if c.MaxAlertQueueSize <= 0 {
		return errors.New("MaxAlertQueueSize must be positive")
	}
	// if c.IPRateLimit <= 0 { // Removed validation for old fields
	// 	return errors.New("IPRateLimit must be positive")
	// }
	// if c.IPRateBurst <= 0 { // Removed validation for old fields
	// 	return errors.New("IPRateBurst must be positive")
	// }
	// if c.MaxIPRateLimiters <= 0 { // Removed validation for old fields
	// 	return errors.New("MaxIPRateLimiters must be positive")
	// }
	if c.MaxRequestSizeMB <= 0 {
		return errors.New("MaxRequestSizeMB must be positive")
	}
	if c.Port == "" {
		return errors.New("Port must be set")
	}

	// Validate new shared rate limiter config fields
	if c.GlobalRateLimit <= 0 || c.GlobalRateBurst <= 0 {
		return errors.New("Global rate limit parameters must be positive")
	}
	if c.IPRateLimit <= 0 || c.IPRateBurst <= 0 || c.MaxIPRateLimiters <= 0 {
		return errors.New("IP rate limit parameters must be positive")
	}

	return nil
}

func Default() *ServiceConfig {
	return &ServiceConfig{
		MaxUsageQueueSize:    10000,
		MaxAlertQueueSize:    1000,
		WorkerPoolSize:       100,
		AlertWorkerPoolSize:  10,
		ShutdownTimeout:      30 * time.Second,
		RequestTimeout:       15 * time.Second,
		EnableCircuitBreaker: true,
		Port:                 "8080",
		MaxRequestSizeMB:     1, // 1 MB

		// Default for shared rate limiter config fields
		GlobalRateLimit:   rate.Limit(100), // Example: 100 req/sec global
		GlobalRateBurst:   200,
		IPRateLimit:       rate.Limit(5), // Example: 5 req/sec per IP
		IPRateBurst:       10,
		MaxIPRateLimiters: 10000,
	}
}
