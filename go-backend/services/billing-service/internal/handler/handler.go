package handler

import (
	"net/http"

	"github.com/rs/zerolog"
)

type Service interface {
	HealthCheck() error
	GetInvoice(id string) (interface{}, error)
}

type Handler struct {
	service Service
	log     *zerolog.Logger
}

func New(svc Service, log *zerolog.Logger) *Handler {
	return &Handler{service: svc, log: log}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if err := h.service.HealthCheck(); err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

func (h *Handler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"invoice":"demo"}`))
}
