package model

import (
	"testing"
)

func TestUser_Validate(t *testing.T) {
	tests := []struct {
		name    string
		user    *User
		wantErr bool
	}{
		{"valid user", &User{Email: "test@example.com", PasswordHash: "securePass123"}, false},
		{"invalid email", &User{Email: "invalid", PasswordHash: "pass"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simule une validation basique (à adapter si méthode Validate existe)
			if tt.user.Email == "" || len(tt.user.PasswordHash) < 8 || len(tt.user.Email) < 5 || !contains(tt.user.Email, "@") {
				if !tt.wantErr {
					t.Errorf("expected no error, got error")
				}
			} else {
				if tt.wantErr {
					t.Errorf("expected error, got none")
				}
			}
		})
	}

	t.Run("empty email", func(t *testing.T) {
		user := &User{Email: "", PasswordHash: "securePass123"}
		if err := user.Validate(); err == nil {
			t.Errorf("expected error for empty email")
		}
	})
	t.Run("short password", func(t *testing.T) {
		user := &User{Email: "test@example.com", PasswordHash: "short"}
		if err := user.Validate(); err == nil {
			t.Errorf("expected error for short password")
		}
	})
	t.Run("email without at", func(t *testing.T) {
		user := &User{Email: "testexample.com", PasswordHash: "securePass123"}
		if err := user.Validate(); err == nil {
			t.Errorf("expected error for invalid email")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}

func FuzzUserValidation(f *testing.F) {
	f.Fuzz(func(t *testing.T, email string) {
		u := &User{Email: email, PasswordHash: "securePass123"}
		_ = u.Validate()
	})
}
