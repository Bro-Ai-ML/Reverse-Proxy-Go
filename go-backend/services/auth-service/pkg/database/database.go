package database

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"path/filepath"
	"runtime"
	"time"
)

type DBConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    Name     string
    SSLMode  string
}

func NewPostgres(cfg DBConfig) (*sql.DB, error) {
    connStr := fmt.Sprintf(
        "host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
        cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
    )

    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, fmt.Errorf("error opening database: %w", err)
    }

    // Connection pool settings
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(25)
    db.SetConnMaxLifetime(5 * time.Minute)

    // Verify connection
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := db.PingContext(ctx); err != nil {
        return nil, fmt.Errorf("database ping failed: %w", err)
    }

    return db, nil
}

func RunMigrations(cfg DBConfig) error {
    // Get the path to migrations directory
    _, filename, _, ok := runtime.Caller(0)
    if !ok {
        return fmt.Errorf("unable to get the current file path")
    }
    migrationsPath := filepath.Join(filepath.Dir(filename), "../../../migrations")

    // Create the database URL for migrations
    dbURL := fmt.Sprintf(
        "postgres://%s:%s@%s:%s/%s?sslmode=%s",
        cfg.User,
        cfg.Password,
        cfg.Host,
        cfg.Port,
        cfg.Name,
        cfg.SSLMode,
    )

    // Run migrations
    m, err := migrate.New(
        "file://"+migrationsPath,
        dbURL,
    )
    if err != nil {
        return fmt.Errorf("failed to create migration instance: %w", err)
    }

    // Apply migrations
    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        return fmt.Errorf("failed to apply migrations: %w", err)
    }

    version, dirty, err := m.Version()
    if err != nil && err != migrate.ErrNilVersion {
        return fmt.Errorf("failed to get migration version: %w", err)
    }

    log.Info().
        Uint("version", uint(version)).
        Bool("dirty", dirty).
        Msg("Database migrations applied successfully")

    return nil
}
