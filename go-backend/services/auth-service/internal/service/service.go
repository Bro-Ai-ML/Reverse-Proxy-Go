package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"auth-service/internal/config"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type AuthResponse struct {
	User  *User      `json:"user"`
	Token TokenPair `json:"token"`
}

type Service interface {
	Register(ctx context.Context, email, password, firstName, lastName string) (*AuthResponse, error)
	Login(ctx context.Context, email, password string) (*AuthResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	GetUserByID(ctx context.Context, userID string) (*User, error)
	UpdateUser(ctx context.Context, userID, firstName, lastName string) error
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
	HealthCheck() error
}

type service struct {
	db     *sql.DB
	jwtSvc *JWTService
	log    *zerolog.Logger
}

func New(db *sql.DB, log *zerolog.Logger, jwtCfg config.JWTConfig) Service {
	jwtSvc := NewJWTService(jwtCfg.SecretKey, jwtCfg.AccessTokenTTL, jwtCfg.RefreshTokenTTL)
	return &service{
		db:     db,
		jwtSvc: jwtSvc,
		log:    log,
	}
}

func (s *service) Register(ctx context.Context, email, password, firstName, lastName string) (*AuthResponse, error) {
	// Check if user already exists
	exists, err := s.userExists(ctx, email)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to check if user exists")
		return nil, errors.New("failed to register user")
	}
	if exists {
		return nil, errors.New("user with this email already exists")
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to hash password")
		return nil, errors.New("failed to register user")
	}

	// Create user
	userID := uuid.New().String()
	now := time.Now()
	user := &User{
		ID:           userID,
		Email:        email,
		PasswordHash: string(hash),
		FirstName:    firstName,
		LastName:     lastName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, first_name, last_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, user.ID, user.Email, user.PasswordHash, user.FirstName, user.LastName, user.CreatedAt, user.UpdatedAt)

	if err != nil {
		s.log.Error().Err(err).Msg("Failed to create user")
		return nil, errors.New("failed to register user")
	}

	// Generate tokens
	accessToken, refreshToken, err := s.jwtSvc.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to generate tokens")
		return nil, errors.New("failed to register user")
	}

	// Don't return password hash in response
	user.PasswordHash = ""

	return &AuthResponse{
		User: user,
		Token: TokenPair{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		},
	}, nil
}

func (s *service) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	// Get user by email
	user := &User{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, first_name, last_name, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("invalid email or password")
		}
		s.log.Error().Err(err).Msg("Failed to get user by email")
		return nil, errors.New("failed to login")
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid email or password")
	}

	// Generate tokens
	accessToken, refreshToken, err := s.jwtSvc.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to generate tokens")
		return nil, errors.New("failed to login")
	}

	// Don't return password hash in response
	user.PasswordHash = ""

	return &AuthResponse{
		User: user,
		Token: TokenPair{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		},
	}, nil
}

func (s *service) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	// Validate refresh token
	claims, err := s.jwtSvc.ValidateToken(refreshToken, RefreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Check if user exists
	user, err := s.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Generate new tokens
	accessToken, refreshToken, err := s.jwtSvc.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to generate tokens")
		return nil, errors.New("failed to refresh token")
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *service) GetUserByID(ctx context.Context, userID string) (*User, error) {
	user := &User{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, first_name, last_name, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		s.log.Error().Err(err).Msg("Failed to get user by ID")
		return nil, errors.New("failed to get user")
	}

	return user, nil
}

func (s *service) UpdateUser(ctx context.Context, userID, firstName, lastName string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET first_name = $1, last_name = $2, updated_at = $3
		WHERE id = $4
	`, firstName, lastName, time.Now(), userID)

	if err != nil {
		s.log.Error().Err(err).Msg("Failed to update user")
		return errors.New("failed to update user")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to get rows affected")
		return errors.New("failed to update user")
	}

	if rowsAffected == 0 {
		return errors.New("user not found")
	}

	return nil
}

func (s *service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	// Get current password hash
	var currentHash string
	err := s.db.QueryRowContext(ctx, `
		SELECT password_hash FROM users WHERE id = $1
	`, userID).Scan(&currentHash)

	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("user not found")
		}
		s.log.Error().Err(err).Msg("Failed to get user password")
		return errors.New("failed to change password")
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(currentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to hash new password")
		return errors.New("failed to change password")
	}

	// Update password
	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = $1, updated_at = $2
		WHERE id = $3
	`, string(newHash), time.Now(), userID)

	if err != nil {
		s.log.Error().Err(err).Msg("Failed to update password")
		return errors.New("failed to change password")
	}

	return nil
}

func (s *service) HealthCheck() error {
	return s.db.PingContext(context.Background())
}

func (s *service) userExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)
	`, email).Scan(&exists)

	if err != nil {
		return false, err
	}

	return exists, nil
}
