package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	"stripe-demo/services/user-service/internal/model"
)

// UserRepository is a PostgreSQL implementation of the UserRepository interface
type UserRepository struct {
	db     *sqlx.DB
	logger *zerolog.Logger
}

// NewPostgresRepository creates a new PostgreSQL user repository
func NewPostgresRepository(db *sqlx.DB, logger *zerolog.Logger) *UserRepository {
	return &UserRepository{
		db:     db,
		logger: logger,
	}
}

// CreateUser creates a new user in the database
func (r *UserRepository) CreateUser(ctx context.Context, user *model.User) error {
	user.CreatedAt = time.Now()
	user.UpdatedAt = user.CreatedAt
	userID := user.ID.String()
	params := map[string]interface{}{
		"user_id":       userID,
		"email":         user.Email,
		"password_hash": user.PasswordHash,
		"first_name":    user.FirstName,
		"last_name":     user.LastName,
		"avatar_url":    user.AvatarURL,
		"bio":           user.Bio,
		"is_active":     user.IsActive,
		"is_verified":   user.IsVerified,
		"created_at":    user.CreatedAt,
		"updated_at":    user.UpdatedAt,
		"roles":         user.Roles,
	}
	query := `
		INSERT INTO users (
			id, email, password_hash, first_name, last_name, avatar_url, bio, 
			is_active, is_verified, created_at, updated_at, roles
		) VALUES (
			:user_id, :email, :password_hash, :first_name, :last_name, :avatar_url, :bio,
			:is_active, :is_verified, :created_at, :updated_at, :roles
		) RETURNING id`
	rows, err := r.db.NamedQueryContext(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return errors.New("no rows returned after user insert")
	}
	var id string
	if err := rows.Scan(&id); err != nil {
		return fmt.Errorf("failed to get created user ID: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by ID
func (r *UserRepository) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, avatar_url, bio,
		       is_active, is_verified, last_login_at, created_at, updated_at, roles
		FROM users 
		WHERE id = $1 AND deleted_at IS NULL`
	return r.getUser(ctx, query, id)
}

// GetUserByEmail retrieves a user by email
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, avatar_url, bio,
		       is_active, is_verified, last_login_at, created_at, updated_at, roles
		FROM users 
		WHERE email = $1 AND deleted_at IS NULL`
	return r.getUser(ctx, query, email)
}

// UpdateUser updates an existing user
func (r *UserRepository) UpdateUser(ctx context.Context, user *model.User) error {
	user.UpdatedAt = time.Now()
	userID := user.ID.String()
	params := map[string]interface{}{
		"id":          userID,
		"email":       user.Email,
		"first_name":  user.FirstName,
		"last_name":   user.LastName,
		"avatar_url":  user.AvatarURL,
		"bio":         user.Bio,
		"is_active":   user.IsActive,
		"is_verified": user.IsVerified,
		"updated_at":  user.UpdatedAt,
		"roles":       user.Roles,
	}
	query := `
		UPDATE users 
		SET 
			email = :email,
			first_name = :first_name,
			last_name = :last_name,
			avatar_url = :avatar_url,
			bio = :bio,
			is_active = :is_active,
			is_verified = :is_verified,
			updated_at = :updated_at,
			roles = :roles
		WHERE id = :id AND deleted_at IS NULL`
	return r.execNamed(ctx, query, params, model.ErrUserNotFound)
}

// DeleteUser soft deletes a user
func (r *UserRepository) DeleteUser(ctx context.Context, id string) error {
	query := `
		UPDATE users 
		SET 
			email = CONCAT(email, '_deleted_', EXTRACT(EPOCH FROM NOW())),
			updated_at = NOW(),
			deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`
	return r.execContext(ctx, query, []interface{}{id}, model.ErrUserNotFound)
}

// ListUsers retrieves a paginated list of users
// TODO: Optimiser ListUsers pour éviter N+1 si ajout de relations (préchargement, jointures, etc.)
func (r *UserRepository) ListUsers(ctx context.Context, limit, offset int) ([]*model.User, error) {
	query := `
		SELECT id, email, first_name, last_name, avatar_url, bio,
		       is_active, is_verified, last_login_at, created_at, updated_at, roles
		FROM users 
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`
	var users []*model.User
	if err := r.db.SelectContext(ctx, &users, query, limit, offset); err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	return users, nil
}

// CountUsers returns the total number of active users
func (r *UserRepository) CountUsers(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	var count int
	if err := r.db.GetContext(ctx, &count, query); err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// UpdateLastLogin updates the last login timestamp for a user
func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	query := `
		UPDATE users 
		SET last_login_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`
	return r.execContext(ctx, query, []interface{}{userID}, model.ErrUserNotFound)
}

// UpdatePassword updates a user's password
func (r *UserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	query := `
		UPDATE users 
		SET password_hash = $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL`
	return r.execContext(ctx, query, []interface{}{passwordHash, userID}, model.ErrUserNotFound)
}

// CreateEmailVerificationToken creates a new email verification token
func (r *UserRepository) CreateEmailVerificationToken(ctx context.Context, userID, token string, expiresAt int64) error {
	return r.upsertToken(ctx, emailVerificationToken, userID, token, expiresAt)
}

// VerifyEmailToken verifies an email verification token and returns the user ID
func (r *UserRepository) VerifyEmailToken(ctx context.Context, token string) (string, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()
	// Get the user ID for the token
	var userID string
	query := `
		SELECT user_id 
		FROM email_verification_tokens 
		WHERE token = $1 AND expires_at > NOW()
		LIMIT 1`
	err = tx.GetContext(ctx, &userID, query, token)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return "", model.ErrInvalidToken
		}
		return "", fmt.Errorf("failed to get email verification token: %w", err)
	}
	// Mark user as verified
	query = `
		UPDATE users 
		SET is_verified = true, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id`
	var verifiedUserID string
	err = tx.GetContext(ctx, &verifiedUserID, query, userID)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return "", model.ErrUserNotFound
		}
		return "", fmt.Errorf("failed to verify user email: %w", err)
	}
	// Delete the used token
	query = `DELETE FROM email_verification_tokens WHERE token = $1`
	_, err = tx.ExecContext(ctx, query, token)
	if err != nil {
		tx.Rollback()
		return "", fmt.Errorf("failed to delete used token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}
	return userID, nil
}

// CreatePasswordResetToken creates a new password reset token
func (r *UserRepository) CreatePasswordResetToken(ctx context.Context, email, token string, expiresAt int64) error {
	// First, get the user ID for the email
	var userID string
	query := `SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL`
	err := r.db.GetContext(ctx, &userID, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Don't reveal that the email doesn't exist
			return nil
		}
		return fmt.Errorf("failed to get user by email: %w", err)
	}
	return r.upsertToken(ctx, passwordResetToken, userID, token, expiresAt)
}

// GetUserByPasswordResetToken retrieves a user by a password reset token
func (r *UserRepository) GetUserByPasswordResetToken(ctx context.Context, token string) (*model.User, error) {
	query := `
		SELECT u.id, u.email, u.password_hash, u.first_name, u.last_name, 
		       u.avatar_url, u.bio, u.is_active, u.is_verified, u.last_login_at, 
		       u.created_at, u.updated_at, u.roles
		FROM users u
		JOIN password_reset_tokens prt ON u.id = prt.user_id
		WHERE prt.token = $1 AND prt.expires_at > NOW() AND u.deleted_at IS NULL`
	return r.getUser(ctx, query, token)
}

// DeletePasswordResetToken deletes a used password reset token
func (r *UserRepository) DeletePasswordResetToken(ctx context.Context, token string) error {
	return r.deleteToken(ctx, passwordResetToken, token)
}

// GetUserRoles retrieves all roles for a user
func (r *UserRepository) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT roles FROM users WHERE id = $1 AND deleted_at IS NULL`
	var roles model.StringArray
	if err := r.db.GetContext(ctx, &roles, query, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	return roles, nil
}

// AddUserRole adds a role to a user. The read-modify-write happens inside a
// transaction with SELECT ... FOR UPDATE: the previous lock-free version
// could lose concurrent role updates.
func (r *UserRepository) AddUserRole(ctx context.Context, userID, role string) error {
	return r.withLockedRoles(ctx, userID, func(roles model.StringArray) (model.StringArray, bool) {
		for _, existing := range roles {
			if existing == role {
				return roles, false // already present, nothing to write
			}
		}
		return append(roles, role), true
	})
}

// RemoveUserRole removes a role from a user (transactional, see AddUserRole).
func (r *UserRepository) RemoveUserRole(ctx context.Context, userID, role string) error {
	return r.withLockedRoles(ctx, userID, func(roles model.StringArray) (model.StringArray, bool) {
		found := false
		for i, existing := range roles {
			if existing == role {
				roles = append(roles[:i], roles[i+1:]...)
				found = true
				break
			}
		}
		return roles, found
	})
}

// withLockedRoles locks the user row, applies mutate to the roles and writes
// them back atomically. mutate returns the new roles and whether a write is
// needed.
func (r *UserRepository) withLockedRoles(ctx context.Context, userID string, mutate func(model.StringArray) (model.StringArray, bool)) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	var roles model.StringArray
	err = tx.GetContext(ctx, &roles,
		`SELECT roles FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, userID)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrUserNotFound
		}
		return fmt.Errorf("failed to lock user roles: %w", err)
	}

	newRoles, changed := mutate(roles)
	if changed {
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET roles = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`,
			newRoles, userID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to update user roles: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit roles transaction: %w", err)
	}
	return nil
}

// Helper function to build WHERE clause with multiple conditions
func buildWhereClause(conditions map[string]interface{}) (string, []interface{}) {
	if len(conditions) == 0 {
		return "", nil
	}

	var where []string
	var args []interface{}
	argNum := 1

	for column, value := range conditions {
		where = append(where, fmt.Sprintf("%s = $%d", column, argNum))
		args = append(args, value)
		argNum++
	}

	return "WHERE " + strings.Join(where, " AND "), args
}

// --- Helper functions for DRY and error handling ---
func (r *UserRepository) getUser(ctx context.Context, query string, arg interface{}) (*model.User, error) {
	var user model.User
	if err := r.db.GetContext(ctx, &user, query, arg); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) execNamed(ctx context.Context, query string, params map[string]interface{}, notFoundErr error) error {
	result, err := r.db.NamedExecContext(ctx, query, params)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return notFoundErr
	}
	return nil
}

func (r *UserRepository) execContext(ctx context.Context, query string, args []interface{}, notFoundErr error) error {
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return notFoundErr
	}
	return nil
}

// --- Modularized token/session management ---
type tokenType string

const (
	emailVerificationToken tokenType = "email_verification_tokens"
	passwordResetToken     tokenType = "password_reset_tokens"
)

func (r *UserRepository) upsertToken(ctx context.Context, ttype tokenType, userID, token string, expiresAt int64) error {
	table := string(ttype)
	query :=
		`INSERT INTO ` + table + ` (user_id, token, expires_at)
		VALUES ($1, $2, to_timestamp($3)::timestamp)
		ON CONFLICT (user_id)
		DO UPDATE SET token = EXCLUDED.token, expires_at = EXCLUDED.expires_at, created_at = NOW()`
	_, err := r.db.ExecContext(ctx, query, userID, token, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to upsert %s: %w", table, err)
	}
	return nil
}

func (r *UserRepository) deleteToken(ctx context.Context, ttype tokenType, token string) error {
	table := string(ttype)
	query := `DELETE FROM ` + table + ` WHERE token = $1`
	_, err := r.db.ExecContext(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", table, err)
	}
	return nil
}

// --- DRY role management ---
func (r *UserRepository) updateUserRoles(ctx context.Context, userID string, roles []string) error {
	query := `UPDATE users SET roles = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, roles, userID)
	if err != nil {
		return fmt.Errorf("failed to update user roles: %w", err)
	}
	return nil
}
