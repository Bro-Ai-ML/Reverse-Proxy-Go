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

func New(db *sql.DB, log *zerolog.Logger, cfg interface{}) *Service {
	return &Service{db: db, log: log}
}

func (s *Service) HealthCheck() error {
	return s.db.PingContext(context.Background())
}

func (s *Service) GetInvoice(id string) (interface{}, error) {
	return map[string]string{"invoice": "demo", "id": id}, nil
}

// TODO: Implémenter la logique métier pour billing-service
