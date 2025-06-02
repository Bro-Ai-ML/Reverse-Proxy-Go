package database

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
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
