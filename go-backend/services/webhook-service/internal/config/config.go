package config

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"golang.org/x/time/rate"
)

// WebhookConfig holds the configuration for the webhook service.
// It includes settings for port, Stripe webhook secret, and rate limiting.
type WebhookConfig struct {
	Port                string
	StripeWebhookSecret string // For verifying webhook signatures
	RequestTimeout      time.Duration
	ShutdownTimeout     time.Duration

	// Shared Rate Limiter Configuration
	GlobalRateLimit  rate.Limit
	GlobalRateBurst  int
	IPRateLimit      rate.Limit
	IPRateBurst      int
	MaxIPLimiters    int
	MaxRequestSizeMB int64 // Max size for incoming webhook payloads
}

// DefaultWebhookConfig returns a default configuration for the webhook service.
func DefaultWebhookConfig() *WebhookConfig {
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if secret == "" {
		slog.Error("STRIPE_WEBHOOK_SECRET is not set. Refusing to start for security reasons!")
		os.Exit(1)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8003" // Default port for webhook service
	}
	return &WebhookConfig{
		Port:                port,
		StripeWebhookSecret: secret,
		RequestTimeout:      20 * time.Second,
		ShutdownTimeout:     30 * time.Second,
		GlobalRateLimit:     rate.Limit(50),
		GlobalRateBurst:     100,
		IPRateLimit:         rate.Limit(10),
		IPRateBurst:         20,
		MaxIPLimiters:       5000,
		MaxRequestSizeMB:    2,
	}
}

// Validate checks if the WebhookConfig is valid.
func (c *WebhookConfig) Validate() error {
	if c.Port == "" {
		return errors.New("port must be set")
	}
	if c.StripeWebhookSecret == "" {
		return errors.New("StripeWebhookSecret must be set for secure webhook processing")
	}
	if c.RequestTimeout <= 0 || c.ShutdownTimeout <= 0 {
		return errors.New("timeout parameters must be positive")
	}
	if c.GlobalRateLimit <= 0 || c.GlobalRateBurst <= 0 || c.IPRateLimit <= 0 || c.IPRateBurst <= 0 || c.MaxIPLimiters <= 0 {
		return errors.New("rate limit parameters must be positive")
	}
	if c.MaxRequestSizeMB <= 0 {
		return errors.New("MaxRequestSizeMB must be positive")
	}
	return nil
}
