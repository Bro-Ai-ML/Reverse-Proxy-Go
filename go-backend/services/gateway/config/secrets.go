package config

import (
	"fmt"
	"io/ioutil"
	"os"
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

func readSecret(envVar string) (string, error) {
	filePath := os.Getenv(envVar + "_FILE")
	if filePath != "" {
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	val := os.Getenv(envVar)
	return val, nil
}
