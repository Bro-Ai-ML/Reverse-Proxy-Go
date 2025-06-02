package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"stripe-demo/services/webhook-service/internal/config"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

// Service handles incoming Stripe webhooks.
// It uses shared middleware for security headers and rate limiting.
type Service struct {
	config *config.WebhookConfig
}

// NewService creates a new webhook Service.
func NewService(cfg *config.WebhookConfig) *Service {
	return &Service{
		config: cfg,
	}
}

// HandleWebhook processes incoming Stripe webhooks.
// It verifies the signature, decodes the event, and logs it.
func (s *Service) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.config.RequestTimeout)
	defer cancel()
	defer r.Body.Close()

	r.Body = http.MaxBytesReader(w, r.Body, s.config.MaxRequestSizeMB*1024*1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			slog.WarnContext(ctx, "Webhook payload too large", "error", err, "limit_bytes", maxBytesError.Limit, "remote_addr", r.RemoteAddr)
			http.Error(w, "Payload too large", http.StatusRequestEntityTooLarge)
		} else {
			slog.WarnContext(ctx, "Failed to read webhook request body", "error", err, "remote_addr", r.RemoteAddr)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		}
		return
	}

	// Verify webhook signature
	signatureHeader := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEventWithOptions(body, signatureHeader, s.config.StripeWebhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true, // Or handle versioning explicitly
	})
	if err != nil {
		slog.WarnContext(ctx, "Webhook signature verification failed", "error", err, "remote_addr", r.RemoteAddr)
		http.Error(w, "Signature verification failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	slog.InfoContext(ctx, "Received Stripe webhook", "event_id", event.ID, "event_type", event.Type, "api_version", event.APIVersion)

	// Process the event (example: log based on type)
	switch event.Type {
	case "payment_intent.succeeded":
		var paymentIntent stripe.PaymentIntent
		if err := json.Unmarshal(event.Data.Raw, &paymentIntent); err != nil {
			slog.ErrorContext(ctx, "Error unmarshalling payment_intent.succeeded data", "error", err, "event_id", event.ID)
		} else {
			slog.InfoContext(ctx, "PaymentIntent succeeded", "payment_intent_id", paymentIntent.ID, "amount", paymentIntent.Amount, "currency", paymentIntent.Currency)
		}
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		var subscription stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
			slog.ErrorContext(ctx, "Error unmarshalling subscription data", "error", err, "event_id", event.ID, "event_type", event.Type)
		} else {
			slog.InfoContext(ctx, "Subscription event", "subscription_id", subscription.ID, "customer_id", subscription.Customer.ID, "status", subscription.Status, "event_type", event.Type)
		}
	default:
		slog.InfoContext(ctx, "Unhandled webhook event type", "event_type", event.Type, "event_id", event.ID)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "event_received": event.ID})
}

// HealthCheck provides a simple health status.
func (s *Service) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"service":   "webhook-service",
		"timestamp": time.Now().Format(time.RFC3339Nano),
	})
}
