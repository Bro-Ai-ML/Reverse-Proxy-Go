package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/stripe-ecosystem/shared/contracts"
	"stripe-demo/services/payment-service/internal"
	"stripe-demo/services/payment-service/internal/config"
)

// Service handles HTTP requests for the payment service.
type Service struct {
	provider internal.PaymentProvider // Renamed from payments for clarity
	config   *config.PaymentConfig
}

// NewService creates a new payment Service.
func NewService(cfg *config.PaymentConfig, provider internal.PaymentProvider) *Service {
	return &Service{
		provider: provider,
		config:   cfg,
	}
}

func (s *Service) CreatePayment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.config.RequestTimeout)
	defer cancel()
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024) // 1MB limit for now

	var req contracts.PaymentRequest
	// Attempt to decode first. If it fails, check if it was due to MaxBytesError.
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		var syntaxError *json.SyntaxError // Add this to catch JSON parsing issues specifically
		if errors.As(err, &maxBytesError) {
			slog.WarnContext(ctx, "Request body too large", "error", err, "limit_bytes", maxBytesError.Limit, "remote_addr", r.RemoteAddr)
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		} else if errors.As(err, &syntaxError) {
			slog.WarnContext(ctx, "Malformed JSON request body for payment", "error", err, "offset", syntaxError.Offset, "remote_addr", r.RemoteAddr)
			http.Error(w, "Malformed JSON: "+err.Error(), http.StatusBadRequest)
		} else {
			// Handle other Decode errors (e.g., io.EOF for empty body, type mismatches not caught by JSON schema)
			slog.WarnContext(ctx, "Invalid request body for payment", "error", err, "remote_addr", r.RemoteAddr)
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		}
		return
	}

	if err := validatePaymentRequest(&req); err != nil {
		slog.WarnContext(ctx, "Invalid payment request data", "error", err, "customer_id", req.CustomerID, "amount", req.Amount, "currency", req.Currency)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	intent, err := s.provider.CreatePaymentIntent(ctx, req.Amount, req.Currency, req.CustomerID, req.Metadata) // Use s.provider
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create payment intent", "error", err, "customer_id", req.CustomerID, "amount", req.Amount)
		http.Error(w, "Failed to create payment", http.StatusInternalServerError)
		return
	}
	slog.InfoContext(ctx, "Payment intent created successfully", "payment_intent_id", intent.ID, "customer_id", req.CustomerID, "amount", intent.Amount)

	payment := &contracts.Payment{
		ID:          intent.ID,
		CustomerID:  req.CustomerID,
		Amount:      intent.Amount,
		Currency:    string(intent.Currency),
		Status:      string(intent.Status),
		StripeID:    intent.ID,
		Description: req.Description,
		Metadata:    req.Metadata,
		CreatedAt:   time.Unix(intent.Created, 0),
		UpdatedAt:   time.Now(),
	}

	response := &contracts.PaymentResponse{Payment: payment, ClientSecret: intent.ClientSecret}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func validatePaymentRequest(req *contracts.PaymentRequest) error {
	if req.Amount <= 0 {
		return errors.New("Amount must be positive")
	}
	if req.Currency == "" {
		return errors.New("Currency is required")
	}
	if req.CustomerID == "" {
		// Depending on policy, CustomerID might be optional or required.
		// For now, allowing it to be empty as per original logic.
		// slog.DebugContext(context.Background(), "CustomerID is empty in payment request")
	}
	return nil
}

func (s *Service) GetPayment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.config.RequestTimeout)
	defer cancel()

	vars := mux.Vars(r)
	paymentID := vars["id"]
	if strings.TrimSpace(paymentID) == "" {
		slog.WarnContext(ctx, "Payment ID is required for GetPayment")
		http.Error(w, "Payment ID is required", http.StatusBadRequest)
		return
	}

	intent, err := s.provider.GetPaymentIntent(ctx, paymentID) // Use s.provider
	if err != nil {
		slog.ErrorContext(ctx, "Failed to retrieve payment intent", "error", err, "payment_intent_id", paymentID)
		http.Error(w, "Failed to retrieve payment", http.StatusInternalServerError)
		return
	}
	slog.InfoContext(ctx, "Payment intent retrieved successfully", "payment_intent_id", intent.ID, "status", intent.Status)

	customerID := ""
	if intent.Customer != nil {
		customerID = intent.Customer.ID
	}

	payment := &contracts.Payment{
		ID:         intent.ID,
		CustomerID: customerID,
		Amount:     intent.Amount,
		Currency:   string(intent.Currency),
		Status:     string(intent.Status),
		StripeID:   intent.ID,
		Metadata:   intent.Metadata,
		CreatedAt:  time.Unix(intent.Created, 0),
		UpdatedAt:  time.Unix(intent.Created, 0), // Should likely be intent.Modified or time.Now() if that's more appropriate
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payment)
}

func (s *Service) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"service":   "payment-service",
		"timestamp": time.Now().Format(time.RFC3339Nano),
	})
}
