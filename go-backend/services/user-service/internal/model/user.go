package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User représente un utilisateur du système
type User struct {
	ID           uuid.UUID   `json:"id" db:"id"`
	Email        string      `json:"email" db:"email"`
	PasswordHash string      `json:"-" db:"password_hash"`
	FirstName    string      `json:"first_name" db:"first_name"`
	LastName     string      `json:"last_name" db:"last_name"`
	AvatarURL    string      `json:"avatar_url,omitempty" db:"avatar_url"`
	Bio          string      `json:"bio,omitempty" db:"bio"`
	IsActive     bool        `json:"is_active" db:"is_active"`
	IsVerified   bool        `json:"is_verified" db:"is_verified"`
	LastLoginAt  *time.Time  `json:"last_login_at,omitempty" db:"last_login_at"`
	CreatedAt    time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at" db:"updated_at"`
	DeletedAt    *time.Time  `json:"-" db:"deleted_at"`
	Roles        StringArray `json:"roles,omitempty" db:"roles"`

	// RGPD Fields
	DataPortability  bool   `json:"data_portability" db:"data_portability"`
	MarketingConsent bool   `json:"marketing_consent" db:"marketing_consent"`
	TermsAccepted    bool   `json:"terms_accepted" db:"terms_accepted"`
	TermsVersion     string `json:"terms_version" db:"terms_version"`
	LastIP           string `json:"-" db:"last_ip"`
	UserAgent        string `json:"-" db:"user_agent"`
}

// UserCreate représente les données nécessaires à la création d'un utilisateur
type UserCreate struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name" validate:"required"`
}

// UserUpdate représente les champs modifiables d'un utilisateur
type UserUpdate struct {
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	Bio       *string `json:"bio,omitempty"`
}

// UserLogin représente les identifiants de connexion
type UserLogin struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// ChangePasswordRequest représente une demande de changement de mot de passe
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// ResetPasswordRequest représente une demande de réinitialisation de mot de passe
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// ForgotPasswordRequest représente une demande d'oubli de mot de passe
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// VerifyEmailRequest représente une demande de vérification d'email
type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required"`
}

// AuthResponse représente la réponse d'authentification
type AuthResponse struct {
	User         *User  `json:"user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// UserResponse represents a user response object (without sensitive data)
type UserResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Bio         string    `json:"bio,omitempty"`
	IsActive    bool      `json:"is_active"`
	IsVerified  bool      `json:"is_verified"`
	LastLoginAt time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Roles       []string  `json:"roles,omitempty"`
}

// ToResponse converts a User to a UserResponse
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:          u.ID,
		Email:       u.Email,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		AvatarURL:   u.AvatarURL,
		Bio:         u.Bio,
		IsActive:    u.IsActive,
		IsVerified:  u.IsVerified,
		LastLoginAt: *u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
		Roles:       u.Roles,
	}
}

// StringArray is a custom type for handling string arrays in the database
type StringArray []string

// Scan implements the sql.Scanner interface
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("invalid type for StringArray")
	}

	if len(bytes) == 0 {
		*s = []string{}
		return nil
	}

	return json.Unmarshal(bytes, s)
}

// Value implements the driver.Valuer interface
func (s StringArray) Value() (driver.Value, error) {
	if s == nil || len(s) == 0 {
		return "[]", nil
	}
	return json.Marshal(s)
}

// HasRole checks if the user has the specified role
func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// RGPDData returns a map containing all personal data for GDPR export
func (u *User) RGPDData() map[string]interface{} {
	return map[string]interface{}{
		"user": map[string]interface{}{
			"id":          u.ID,
			"email":       u.Email,
			"first_name":  u.FirstName,
			"last_name":   u.LastName,
			"created_at":  u.CreatedAt,
			"last_login":  u.LastLoginAt,
			"is_verified": u.IsVerified,
			"is_active":   u.IsActive,
		},
		"preferences": map[string]interface{}{
			"marketing_consent": u.MarketingConsent,
			"data_portability":  u.DataPortability,
			"terms_accepted":    u.TermsAccepted,
			"terms_version":     u.TermsVersion,
		},
	}
}

// Anonymize removes all personal data from the user
func (u *User) Anonymize() {
	u.Email = "deleted-" + u.ID.String() + "@deleted.local"
	u.FirstName = "Anonyme"
	u.LastName = "Utilisateur"
	u.PasswordHash = ""
	u.AvatarURL = ""
	u.Bio = ""
	u.IsActive = false
	u.MarketingConsent = false
	u.DataPortability = false
	u.TermsAccepted = false
	u.LastIP = ""
	u.UserAgent = ""
	now := time.Now()
	u.DeletedAt = &now
}

// HasAnyRole checks if the user has any of the specified roles
func (u *User) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if u.HasRole(role) {
			return true
		}
	}
	return false
}

// HasAllRoles checks if the user has all of the specified roles
func (u *User) HasAllRoles(roles ...string) bool {
	for _, role := range roles {
		if !u.HasRole(role) {
			return false
		}
	}
	return true
}

var (
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidToken = errors.New("invalid token")
)

func (u *User) Validate() error {
	if u.Email == "" || len(u.Email) < 5 || !strings.Contains(u.Email, "@") {
		return errors.New("invalid email")
	}
	if len(u.PasswordHash) < 8 {
		return errors.New("password too short")
	}
	return nil
}
