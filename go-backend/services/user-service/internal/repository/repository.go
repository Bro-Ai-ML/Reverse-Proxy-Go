package repository

import (
	"context"

	"stripe-demo/services/user-service/internal/model"
)

// UserRepository defines the interface for user data access
type UserRepository interface {
	// User management
	CreateUser(ctx context.Context, user *model.User) error
	GetUserByID(ctx context.Context, id string) (*model.User, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	UpdateUser(ctx context.Context, user *model.User) error
	DeleteUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context, limit, offset int) ([]*model.User, error)
	CountUsers(ctx context.Context) (int, error)

	// Authentication
	UpdateLastLogin(ctx context.Context, userID string) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error

	// Email verification
	CreateEmailVerificationToken(ctx context.Context, userID, token string, expiresAt int64) error
	VerifyEmailToken(ctx context.Context, token string) (string, error)

	// Password reset
	CreatePasswordResetToken(ctx context.Context, email, token string, expiresAt int64) error
	GetUserByPasswordResetToken(ctx context.Context, token string) (*model.User, error)
	DeletePasswordResetToken(ctx context.Context, token string) error

	// User roles
	GetUserRoles(ctx context.Context, userID string) ([]string, error)
	AddUserRole(ctx context.Context, userID, role string) error
	RemoveUserRole(ctx context.Context, userID, role string) error
}
