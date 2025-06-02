package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	t.Run("should load default config", func(t *testing.T) {
		os.Unsetenv("PORT")
		os.Unsetenv("JWT_SECRET_KEY")
		cfg, err := Load()
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
	})

	t.Run("should override with env vars", func(t *testing.T) {
		os.Setenv("PORT", "3001")
		os.Setenv("JWT_SECRET_KEY", "super-strong-key-1234567890")
		cfg, _ := Load()
		assert.Equal(t, "3001", cfg.Server.Port)
		os.Unsetenv("PORT")
		os.Unsetenv("JWT_SECRET_KEY")
	})

	t.Run("should panic on invalid JWT secret", func(t *testing.T) {
		os.Setenv("JWT_SECRET_KEY", "weak")
		assert.Panics(t, func() { Load() })
		os.Unsetenv("JWT_SECRET_KEY")
	})
}
