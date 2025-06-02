package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestDefaultConfig(t *testing.T) {
	// Save and restore environment variables
	tests := []struct {
		name     string
		env      map[string]string
		validate func(*testing.T, *PaymentConfig)
	}{
		{
			name: "default values",
			env: map[string]string{
				"PORT":              "",
				"STRIPE_SECRET_KEY": "sk_test_123",
			},
			validate: func(t *testing.T, cfg *PaymentConfig) {
				assert.Equal(t, "8001", cfg.Port)
				assert.Equal(t, 15*time.Second, cfg.RequestTimeout)
				assert.Equal(t, 30*time.Second, cfg.ShutdownTimeout)
				assert.Equal(t, "sk_test_123", cfg.StripeSecretKey)
				assert.Equal(t, rate.Limit(100), cfg.GlobalRateLimit)
				assert.Equal(t, 200, cfg.GlobalRateBurst)
				assert.Equal(t, rate.Limit(20), cfg.IPRateLimit)
				assert.Equal(t, 40, cfg.IPRateBurst)
				assert.Equal(t, 10000, cfg.MaxIPLimiters)
			},
		},
		{
			name: "custom port",
			env: map[string]string{
				"PORT":              "3000",
				"STRIPE_SECRET_KEY": "sk_test_123",
			},
			validate: func(t *testing.T, cfg *PaymentConfig) {
				assert.Equal(t, "3000", cfg.Port)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			defer func() {
				// Cleanup
				for k := range tt.env {
					os.Unsetenv(k)
				}
			}()

			// Test
			cfg := DefaultConfig()
			tt.validate(t, cfg)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *PaymentConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: &PaymentConfig{
				Port:            "8080",
				StripeSecretKey: "sk_test_123",
				RequestTimeout:  10 * time.Second,
				ShutdownTimeout: 30 * time.Second,
				GlobalRateLimit: 100,
				GlobalRateBurst: 200,
				IPRateLimit:     20,
				IPRateBurst:     40,
				MaxIPLimiters:   10000,
			},
			wantErr: false,
		},
		{
			name: "missing port",
			cfg: &PaymentConfig{
				Port:            "",
				StripeSecretKey: "sk_test_123",
				RequestTimeout:  10 * time.Second,
			},
			wantErr: true,
			errMsg:  "Port must be set",
		},
		{
			name: "missing stripe key",
			cfg: &PaymentConfig{
				Port:            "8080",
				StripeSecretKey: "",
				RequestTimeout:  10 * time.Second,
			},
			wantErr: true,
			errMsg:  "StripeSecretKey must be set",
		},
		{
			name: "invalid timeouts",
			cfg: &PaymentConfig{
				Port:            "8080",
				StripeSecretKey: "sk_test_123",
				RequestTimeout:  0,
				ShutdownTimeout: 30 * time.Second,
				GlobalRateLimit: 100,
				GlobalRateBurst: 200,
				IPRateLimit:     20,
				IPRateBurst:     40,
				MaxIPLimiters:   10000,
			},
			wantErr: true,
			errMsg:  "Timeout parameters must be positive",
		},
		{
			name: "invalid rate limits",
			cfg: &PaymentConfig{
				Port:            "8080",
				StripeSecretKey: "sk_test_123",
				RequestTimeout:  10 * time.Second,
				ShutdownTimeout: 30 * time.Second,
				GlobalRateLimit: 0,
				GlobalRateBurst: 200,
				IPRateLimit:     20,
				IPRateBurst:     40,
				MaxIPLimiters:   10000,
			},
			wantErr: true,
			errMsg:  "Rate limit parameters must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
