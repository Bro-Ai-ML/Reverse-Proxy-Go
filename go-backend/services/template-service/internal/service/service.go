package service

import (
	"context"
	"database/sql"

	"github.com/rs/zerolog"
)

type Service struct {
	db  *sql.DB
	log *zerolog.Logger
}

func New(db *sql.DB, log *zerolog.Logger) *Service {
	return &Service{
		db:  db,
		log: log,
	}
}

func (s *Service) HealthCheck() error {
	return s.db.PingContext(context.Background())
}

func (s *Service) GetResource(id string) (interface{}, error) {
	return map[string]string{"id": id, "resource": "demo"}, nil
}

func (s *Service) CreateResource(data interface{}) error {
	return nil // Simule la création
}
