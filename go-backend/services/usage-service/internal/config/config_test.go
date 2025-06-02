package config

import "testing"

func TestDefaultAndValidate(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid default config, got error: %v", err)
	}
}
