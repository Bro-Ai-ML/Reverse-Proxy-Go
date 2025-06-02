package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

const (
	DefaultDBMaxOpenConns   = 10
	DefaultDBMaxIdleConns   = 5
	DefaultEmailPort        = 587
	DefaultRGPDRetention    = 365
	DefaultIdleTimeout      = 120 * time.Second
	DefaultShutdownTimeout  = 30 * time.Second
	DefaultJWTAccessMinutes = 15
	DefaultJWTRefreshDays   = 7
	DefaultHoursPerDay      = 24
)

// Config holds all configuration for the application
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	DB     DBConfig     `mapstructure:"database"`
	JWT    JWTConfig    `mapstructure:"jwt"`
	Email  EmailConfig  `mapstructure:"email"`
	CORS   CORSConfig   `mapstructure:"cors"`
	RGPD   RGPDConfig   `mapstructure:"rgpd"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port            string        `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	Environment     string        `mapstructure:"environment"`
	TrustProxy      bool          `mapstructure:"trust_proxy"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// DBConfig holds database configuration
type DBConfig struct {
	Host         string        `mapstructure:"host"`
	Port         string        `mapstructure:"port"`
	User         string        `mapstructure:"user"`
	Password     string        `mapstructure:"password"`
	Name         string        `mapstructure:"name"`
	SSLMode      string        `mapstructure:"sslmode"`
	MaxOpenConns int           `mapstructure:"max_open_conns"`
	MaxIdleConns int           `mapstructure:"max_idle_conns"`
	MaxIdleTime  time.Duration `mapstructure:"max_idle_time"`
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	SecretKey       string        `mapstructure:"secret_key"`
	AccessDuration  time.Duration `mapstructure:"access_duration"`
	RefreshDuration time.Duration `mapstructure:"refresh_duration"`
	Issuer          string        `mapstructure:"issuer"`
}

// EmailConfig holds email configuration
type EmailConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	From         string `mapstructure:"from"`
	TemplatePath string `mapstructure:"template_path"`
	UseTLS       bool   `mapstructure:"use_tls"`
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
	AllowedHeaders []string `mapstructure:"allowed_headers"`
}

// RGPDConfig holds RGPD/GDPR configuration
type RGPDConfig struct {
	DataRetentionDays      int    `mapstructure:"data_retention_days"`
	AnonymizeAfterDelete   bool   `mapstructure:"anonymize_after_delete"`
	DefaultTermsVersion    string `mapstructure:"default_terms_version"`
	RequireExplicitConsent bool   `mapstructure:"require_explicit_consent"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:            "8080",
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     120 * time.Second,
			Environment:     "development",
			TrustProxy:      false,
			ShutdownTimeout: 30 * time.Second,
		},
		DB:    DefaultDBConfig(),
		JWT:   DefaultJWTConfig(),
		Email: DefaultEmailConfig(),
		CORS:  DefaultCORSConfig(),
		RGPD:  DefaultRGPDConfig(),
	}
}

// DefaultDBConfig returns the default database configuration
func DefaultDBConfig() DBConfig {
	return DBConfig{
		Host:         "localhost",
		Port:         "5432",
		User:         "postgres",
		Password:     "postgres",
		Name:         "user_service",
		SSLMode:      "disable",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
		MaxIdleTime:  5 * time.Minute,
	}
}

// DefaultJWTConfig returns the default JWT configuration
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		SecretKey:       "your-256-bit-secret", // Change this in production
		AccessDuration:  15 * time.Minute,
		RefreshDuration: 7 * 24 * time.Hour, // 1 week
		Issuer:          "user-service",
	}
}

// DefaultEmailConfig returns the default email configuration
func DefaultEmailConfig() EmailConfig {
	return EmailConfig{
		Host:         "smtp.example.com",
		Port:         587,
		Username:     "user@example.com",
		Password:     "password",
		From:         "noreply@example.com",
		TemplatePath: "./templates/email",
		UseTLS:       true,
	}
}

// DefaultCORSConfig returns the default CORS configuration
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"}, // In production, replace with your frontend domains
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}
}

// DefaultRGPDConfig returns the default RGPD configuration
func DefaultRGPDConfig() RGPDConfig {
	return RGPDConfig{
		DataRetentionDays:      365, // 1 year
		AnonymizeAfterDelete:   true,
		DefaultTermsVersion:    "1.0.0",
		RequireExplicitConsent: true,
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvSlice gets an environment variable as a slice, splitting by comma
func getEnvSlice(key string, defaultValue []string) []string {
	value := getEnv(key, "")
	if value == "" {
		return defaultValue
	}
	return strings.Split(value, ",")
}

// ConnectionString returns the database connection string
func (c DBConfig) ConnectionString() string {
	return "host=" + c.Host +
		" port=" + c.Port +
		" user=" + c.User +
		" password=" + c.Password +
		" dbname=" + c.Name +
		" sslmode=" + c.SSLMode
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Load server config
	if port := os.Getenv("PORT"); port != "" {
		cfg.Server.Port = port
	}

	// Load DB config
	if host := os.Getenv("DB_HOST"); host != "" {
		cfg.DB.Host = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		cfg.DB.Port = port
	}
	if user := os.Getenv("DB_USER"); user != "" {
		cfg.DB.User = user
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		cfg.DB.Password = password
	}
	if name := os.Getenv("DB_NAME"); name != "" {
		cfg.DB.Name = name
	}
	if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
		cfg.DB.SSLMode = sslMode
	}

	// Load JWT config
	if secretKey := os.Getenv("JWT_SECRET_KEY"); secretKey != "" {
		cfg.JWT.SecretKey = secretKey
	}
	// Validation stricte de la clé JWT
	if cfg.JWT.SecretKey == "your-256-bit-secret" || cfg.JWT.SecretKey == "" {
		panic("JWT secret key is too weak or missing. Set JWT_SECRET_KEY in env.")
	}

	// Load Email config
	if emailHost := os.Getenv("EMAIL_HOST"); emailHost != "" {
		cfg.Email.Host = emailHost
	}

	// Load RGPD config
	if dataRetentionDays := os.Getenv("RGPD_DATA_RETENTION_DAYS"); dataRetentionDays != "" {
		if days, err := time.ParseDuration(dataRetentionDays + "h"); err == nil {
			cfg.RGPD.DataRetentionDays = int(days.Hours() / 24)
		}
	}

	// Validation stricte de la config
	if cfg.Server.Port == "" || cfg.DB.Host == "" || cfg.DB.User == "" || cfg.DB.Password == "" || cfg.DB.Name == "" {
		return nil, errors.New("Critical config missing: check PORT, DB_HOST, DB_USER, DB_PASSWORD, DB_NAME")
	}

	return &cfg, nil
}
