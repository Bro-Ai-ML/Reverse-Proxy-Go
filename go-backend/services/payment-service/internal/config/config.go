package config

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"golang.org/x/time/rate"
)

// PaymentConfig holds the configuration for the payment service.
type PaymentConfig struct {
	Port            string
	StripeSecretKey string
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration

	// Shared Rate Limiter Configuration
	GlobalRateLimit rate.Limit
	GlobalRateBurst int
	IPRateLimit     rate.Limit
	IPRateBurst     int
	MaxIPLimiters   int
}

// DefaultConfig returns a default configuration for the payment service.
func DefaultConfig() *PaymentConfig {
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		slog.Warn("STRIPE_SECRET_KEY is not set. Payment service may not function correctly.")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8001" // Default port for payment service
	}
	return &PaymentConfig{
		Port:            port,
		StripeSecretKey: stripeKey,
		RequestTimeout:  15 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		GlobalRateLimit: rate.Limit(100),
		GlobalRateBurst: 200,
		IPRateLimit:     rate.Limit(20),
		IPRateBurst:     40,
		MaxIPLimiters:   10000,
	}
}

// Validate checks if the PaymentConfig is valid.
func (c *PaymentConfig) Validate() error {
	if c.Port == "" {
		return errors.New("Port must be set")
	}
	if c.StripeSecretKey == "" {
		return errors.New("StripeSecretKey must be set")
	}
	if c.RequestTimeout <= 0 || c.ShutdownTimeout <= 0 {
		return errors.New("Timeout parameters must be positive")
	}
	if c.GlobalRateLimit <= 0 || c.GlobalRateBurst <= 0 || c.IPRateLimit <= 0 || c.IPRateBurst <= 0 || c.MaxIPLimiters <= 0 {
		return errors.New("Rate limit parameters must be positive")
	}
	return nil
}
