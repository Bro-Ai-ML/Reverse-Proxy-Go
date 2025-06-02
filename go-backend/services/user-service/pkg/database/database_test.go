package database

import (
	"os"
	"testing"
)

func TestNewPostgres(t *testing.T) {
	host := os.Getenv("TEST_DB_HOST")
	if host == "" {
		t.Skip("TEST_DB_HOST not set, skipping DB test")
	}
	cfg := Config{
		Host:     host,
		Port:     os.Getenv("TEST_DB_PORT"),
		User:     os.Getenv("TEST_DB_USER"),
		Password: os.Getenv("TEST_DB_PASSWORD"),
		Name:     os.Getenv("TEST_DB_NAME"),
		SSLMode:  "disable",
	}
	db, err := NewPostgres(cfg)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	if db == nil {
		t.Fatal("db is nil")
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}
