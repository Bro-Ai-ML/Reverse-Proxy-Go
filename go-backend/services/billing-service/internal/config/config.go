package config

import (
	"os"
	"strconv"
)

type Config struct {
    Port string
    DB   DBConfig
    Auth AuthConfig
    Stripe StripeConfig
}

type DBConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    Name     string
    SSLMode  string
}

type AuthConfig struct {
    JWTSecret string
    PublicKeyURL string
}

type StripeConfig struct {
    SecretKey      string
    WebhookSecret  string
    SuccessURL     string
    CancelURL      string
    DefaultCurrency string
}

func Load() (*Config, error) {
    return &Config{
        Port: getEnv("PORT", "8080"),
        DB: DBConfig{
            Host:     getEnv("DB_HOST", "localhost"),
            Port:     getEnv("DB_PORT", "5432"),
            User:     getEnv("DB_USER", "postgres"),
            Password: getEnv("DB_PASSWORD", "postgres"),
            Name:     getEnv("DB_NAME", "billing"),
            SSLMode:  getEnv("DB_SSLMODE", "disable"),
        },
        Auth: AuthConfig{
            JWTSecret:   getEnv("AUTH_JWT_SECRET", "your-jwt-secret"),
            PublicKeyURL: getEnv("AUTH_PUBLIC_KEY_URL", "http://auth-service:8080/api/v1/auth/public-key"),
        },
        Stripe: StripeConfig{
            SecretKey:      getEnv("STRIPE_SECRET_KEY", ""),
            WebhookSecret:  getEnv("STRIPE_WEBHOOK_SECRET", ""),
            SuccessURL:     getEnv("STRIPE_SUCCESS_URL", "http://localhost:3000/billing/success"),
            CancelURL:      getEnv("STRIPE_CANCEL_URL", "http://localhost:3000/billing/cancel"),
            DefaultCurrency: getEnv("STRIPE_DEFAULT_CURRENCY", "usd"),
        },
    }, nil
}

func getEnv(key, fallback string) string {
    if value, exists := os.LookupEnv(key); exists {
        return value
    }
    return fallback
}

func getEnvAsInt(key string, fallback int) int {
    if value, exists := os.LookupEnv(key); exists {
        if intValue, err := strconv.Atoi(value); err == nil {
            return intValue
        }
    }
    return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
    if value, exists := os.LookupEnv(key); exists {
        if boolValue, err := strconv.ParseBool(value); err == nil {
            return boolValue
        }
    }
    return fallback
}
