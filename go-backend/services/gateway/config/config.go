package config

import (
	"time"
)

type Config struct {
	Server struct {
		Port         string        `yaml:"port"`
		ReadTimeout  time.Duration `yaml:"read_timeout"`
		WriteTimeout time.Duration `yaml:"write_timeout"`
		IdleTimeout  time.Duration `yaml:"idle_timeout"`
	} `yaml:"server"`

	Auth struct {
		JWTPrivateKeyPath string        `yaml:"jwt_private_key_path"`
		JWTPublicKeyPath  string        `yaml:"jwt_public_key_path"`
		TokenDuration     time.Duration `yaml:"token_duration"`
		RefreshDuration   time.Duration `yaml:"refresh_duration"`
	} `yaml:"auth"`
}
