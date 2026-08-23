package config

import (
	"fmt"
	"os"
	"strings"
)

type Secrets struct {
	JWTSecret   string
	ServiceAKey string
}

func LoadSecrets() (*Secrets, error) {
	jwtSecret, err := readSecret("JWT_SECRET")
	if err != nil {
		return nil, fmt.Errorf("JWT secret: %w", err)
	}
	serviceAKey, err := readSecret("SERVICE_A_KEY")
	if err != nil {
		return nil, fmt.Errorf("ServiceA key: %w", err)
	}
	return &Secrets{
		JWTSecret:   jwtSecret,
		ServiceAKey: serviceAKey,
	}, nil
}

// readSecret reads a secret either from a file (ENV_VAR_FILE, Docker-secret
// style) or directly from the environment variable. File contents are
// trimmed so trailing newlines (common with mounted secrets) don't leak into
// the value.
func readSecret(envVar string) (string, error) {
	filePath := os.Getenv(envVar + "_FILE")
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}
	return os.Getenv(envVar), nil
}
