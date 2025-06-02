package config

import (
	"os"
	"strconv"
)

type Config struct {
    Port string
    DB   DBConfig
    JWT  JWTConfig
}

type DBConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    Name     string
    SSLMode  string
}

type JWTConfig struct {
    SecretKey       string
    AccessTokenTTL  int
    RefreshTokenTTL int
}

func Load() (*Config, error) {
    return &Config{
        Port: getEnv("PORT", "8080"),
        DB: DBConfig{
            Host:     getEnv("DB_HOST", "localhost"),
            Port:     getEnv("DB_PORT", "5432"),
            User:     getEnv("DB_USER", "postgres"),
            Password: getEnv("DB_PASSWORD", "postgres"),
            Name:     getEnv("DB_NAME", "auth"),
            SSLMode:  getEnv("DB_SSLMODE", "disable"),
        },
        JWT: JWTConfig{
            SecretKey:       getEnv("JWT_SECRET_KEY", "default-secret-key"),
            AccessTokenTTL:  getEnvAsInt("JWT_ACCESS_TOKEN_TTL", 15), // 15 minutes
            RefreshTokenTTL: getEnvAsInt("JWT_REFRESH_TOKEN_TTL", 10080), // 7 days in minutes
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
