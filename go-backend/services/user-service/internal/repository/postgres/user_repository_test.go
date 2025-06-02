//go:build integration
// +build integration

package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"stripe-demo/services/user-service/internal/model"
	db "stripe-demo/services/user-service/pkg/database"
	"stripe-demo/services/user-service/pkg/logger"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *db.Postgres {
	host := os.Getenv("TEST_DB_HOST")
	if host == "" {
		t.Skip("TEST_DB_HOST not set")
	}
	cfg := db.Config{
		Host:     host,
		Port:     os.Getenv("TEST_DB_PORT"),
		User:     os.Getenv("TEST_DB_USER"),
		Password: os.Getenv("TEST_DB_PASSWORD"),
		Name:     os.Getenv("TEST_DB_NAME"),
		SSLMode:  "disable",
	}
	database, err := db.NewPostgres(cfg, logger.New())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Créer la table de test si elle n'existe pas
	_, err = database.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			first_name TEXT,
			last_name TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);
	`)
	require.NoError(t, err)

	// Nettoyer la table avant chaque test
	_, err = database.Exec("TRUNCATE TABLE users CASCADE")
	require.NoError(t, err)

	return database
}

func TestUserRepository_CreateAndGet(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPostgresRepository(db, logger.New())

	t.Run("create and get user", func(t *testing.T) {
		userID := uuid.New()
		now := time.Now()

		user := &model.User{
			ID:           userID,
			Email:        "test@example.com",
			PasswordHash: "hashedpassword123",
			FirstName:    "Test",
			LastName:     "User",
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		err := repo.CreateUser(context.Background(), user)
		require.NoError(t, err)

		// Vérifier que l'utilisateur peut être récupéré
		found, err := repo.GetUserByID(context.Background(), userID.String())
		require.NoError(t, err)
		assert.Equal(t, user.Email, found.Email)
		assert.Equal(t, user.FirstName, found.FirstName)
	})
}

func TestUserRepository_CreateAndGetUser(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPostgresRepository(db, zerolog.Nop())
	user := &model.User{
		ID:        uuid.New(),
		Email:     "test@example.com",
		FirstName: "Test",
		LastName:  "User",
	}
	err := repo.CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	found, err := repo.GetUserByID(context.Background(), user.ID.String())
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if found.Email != user.Email {
		t.Errorf("expected %s, got %s", user.Email, found.Email)
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
