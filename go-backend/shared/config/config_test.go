package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Save and clear environment variables
	oldPort := os.Getenv("PORT")
	oldShutdownTimeout := os.Getenv("SHUTDOWN_TIMEOUT")
	defer func() {
		_ = os.Setenv("PORT", oldPort)
		_ = os.Setenv("SHUTDOWN_TIMEOUT", oldShutdownTimeout)
	}()

	// Clear environment variables for this test
	_ = os.Unsetenv("PORT")
	_ = os.Unsetenv("SHUTDOWN_TIMEOUT")

	// Test
	cfg := Load()

	// Assert
	assert.Equal(t, "8080", cfg.Port, "default port should be 8080")
	assert.Equal(t, 10*time.Second, cfg.ShutdownTimeout, "default shutdown timeout should be 10s")
}

func TestLoad_CustomValues(t *testing.T) {
	// Save and clear environment variables
	oldPort := os.Getenv("PORT")
	oldShutdownTimeout := os.Getenv("SHUTDOWN_TIMEOUT")
	defer func() {
		_ = os.Setenv("PORT", oldPort)
		_ = os.Setenv("SHUTDOWN_TIMEOUT", oldShutdownTimeout)
	}()

	// Set custom values
	_ = os.Setenv("PORT", "3000")
	_ = os.Setenv("SHUTDOWN_TIMEOUT", "30")

	// Test
	cfg := Load()

	// Assert
	assert.Equal(t, "3000", cfg.Port, "port should be set from environment")
	assert.Equal(t, 30*time.Second, cfg.ShutdownTimeout, "shutdown timeout should be set from environment")
}

func TestLoad_InvalidShutdownTimeout(t *testing.T) {
	// Save and clear environment variables
	oldShutdownTimeout := os.Getenv("SHUTDOWN_TIMEOUT")
	defer func() {
		_ = os.Setenv("SHUTDOWN_TIMEOUT", oldShutdownTimeout)
	}()

	// Set invalid shutdown timeout
	_ = os.Setenv("SHUTDOWN_TIMEOUT", "invalid")


	// Test
	cfg := Load()

	// Assert - should use default value when invalid
	assert.Equal(t, 10*time.Second, cfg.ShutdownTimeout, "should use default shutdown timeout when invalid")
}

func TestLoad_EmptyPort(t *testing.T) {
	// Save and clear environment variables
	oldPort := os.Getenv("PORT")
	defer func() {
		_ = os.Setenv("PORT", oldPort)
	}()

	// Set empty port
	_ = os.Setenv("PORT", "")


	// Test
	cfg := Load()

	// Assert - should use default port when empty
	assert.Equal(t, "8080", cfg.Port, "should use default port when empty")
}

func TestLoad_ZeroShutdownTimeout(t *testing.T) {
	// Save and clear environment variables
	oldShutdownTimeout := os.Getenv("SHUTDOWN_TIMEOUT")
	defer func() {
		_ = os.Setenv("SHUTDOWN_TIMEOUT", oldShutdownTimeout)
	}()

	// Set zero shutdown timeout
	_ = os.Setenv("SHUTDOWN_TIMEOUT", "0")


	// Test
	cfg := Load()

	// Assert - should use the zero value (0s) when SHUTDOWN_TIMEOUT is set to 0
	assert.Equal(t, 0*time.Second, cfg.ShutdownTimeout, "should use 0s when SHUTDOWN_TIMEOUT is set to 0")
}
