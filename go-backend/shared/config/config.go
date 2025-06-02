package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	ShutdownTimeout time.Duration
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	shutdownTimeout := 10 * time.Second

	if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := strconv.Atoi(v); err == nil {
			shutdownTimeout = time.Duration(d) * time.Second
		}
	}

	return Config{
		Port:            port,
		ShutdownTimeout: shutdownTimeout,
	}
}
