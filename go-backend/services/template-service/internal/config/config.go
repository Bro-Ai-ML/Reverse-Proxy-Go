package config

import "os"

type Config struct {
    Port string
    DB   DBConfig
}

type DBConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    Name     string
    SSLMode  string
}

func Load() (*Config, error) {
    return &Config{
        Port: getEnv("PORT", "8080"),
        DB: DBConfig{
            Host:     getEnv("DB_HOST", "localhost"),
            Port:     getEnv("DB_PORT", "5432"),
            User:     getEnv("DB_USER", "postgres"),
            Password: getEnv("DB_PASSWORD", "postgres"),
            Name:     getEnv("DB_NAME", "template"),
            SSLMode:  getEnv("DB_SSLMODE", "disable"),
        },
    }, nil
}

func getEnv(key, fallback string) string {
    if value, exists := os.LookupEnv(key); exists {
        return value
    }
    return fallback
}
