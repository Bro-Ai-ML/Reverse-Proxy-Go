package handler

import (
	"encoding/json"
	"net/http"
	
	"github.com/rs/zerolog"
)

type Service interface {
    HealthCheck() error
    GetResource(id string) (interface{}, error)
    CreateResource(data interface{}) error
}

type Handler struct {
    service Service
    log     *zerolog.Logger
}

func New(svc Service, log *zerolog.Logger) *Handler {
    return &Handler{
        service: svc,
        log:     log,
    }
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
    if err := h.service.HealthCheck(); err != nil {
        h.log.Error().Err(err).Msg("Health check failed")
        http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (h *Handler) GetResource(w http.ResponseWriter, r *http.Request) {
    // Implement
    w.WriteHeader(http.StatusNotImplemented)
}

func (h *Handler) CreateResource(w http.ResponseWriter, r *http.Request) {
    // Implement
    w.WriteHeader(http.StatusNotImplemented)
}
