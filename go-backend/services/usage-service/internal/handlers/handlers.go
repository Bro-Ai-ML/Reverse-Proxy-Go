package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/stripe-ecosystem/services/usage-service/internal/config"
	"github.com/stripe-ecosystem/shared/contracts"
)

type WorkerPool interface {
	Submit(event contracts.UsageEvent) bool
	Stats() (processed, errors uint64)
}

type Handler struct {
	pool   WorkerPool
	config *config.ServiceConfig
}

func New(pool WorkerPool, cfg *config.ServiceConfig) *Handler {
	return &Handler{
		pool:   pool,
		config: cfg,
	}
}

func (h *Handler) TrackUsage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.config.RequestTimeout)
	defer cancel()
	defer r.Body.Close()

	r.Body = http.MaxBytesReader(w, r.Body, h.config.MaxRequestSizeMB*1024*1024)

	var event contracts.UsageEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		slog.WarnContext(ctx, "Failed to decode usage event", "error", err, "remote_addr", r.RemoteAddr)
		http.Error(w, "Invalid usage event", http.StatusBadRequest)
		return
	}

	if err := validateUsageEvent(event); err != nil {
		slog.WarnContext(ctx, "Invalid usage event data", "error", err, "customer_id", event.CustomerID, "subscription_item_id", event.SubscriptionItemID)
		http.Error(w, fmt.Sprintf("Invalid event: %v", err), http.StatusBadRequest)
		return
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	select {
	case <-ctx.Done():
		slog.WarnContext(ctx, "Request timeout before submitting to pool", "customer_id", event.CustomerID, "subscription_item_id", event.SubscriptionItemID)
		http.Error(w, "Request timeout", http.StatusRequestTimeout)
		return
	default:
		if !h.pool.Submit(event) {
			slog.ErrorContext(ctx, "Failed to submit usage event to pool (overloaded)", "customer_id", event.CustomerID, "subscription_item_id", event.SubscriptionItemID)
			http.Error(w, "Service overloaded", http.StatusServiceUnavailable)
			return
		}
	}
	slog.InfoContext(ctx, "Usage event accepted", "customer_id", event.CustomerID, "subscription_item_id", event.SubscriptionItemID, "quantity", event.Quantity)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "accepted",
		"timestamp": time.Now(),
		"tracked":   true,
	})
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	processed, errors := h.pool.Stats()

	stats := map[string]interface{}{
		"processed_events": processed,
		"error_count":      errors,
		"success_rate":     calculateSuccessRate(processed, errors),
		"timestamp":        time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func validateUsageEvent(event contracts.UsageEvent) error {
	if event.CustomerID == "" {
		return fmt.Errorf("customer_id required")
	}
	if event.SubscriptionItemID == "" {
		return fmt.Errorf("subscription_item_id required")
	}
	if event.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	return nil
}

func calculateSuccessRate(processed, errors uint64) float64 {
	total := processed + errors
	if total == 0 {
		return 100.0
	}
	return float64(processed) / float64(total) * 100.0
}
