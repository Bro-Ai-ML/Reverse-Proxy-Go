package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"stripe-demo/shared/middleware"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/stripe-ecosystem/shared/contracts"
	stripeclient "github.com/stripe-ecosystem/shared/stripe-client"
	"golang.org/x/time/rate"
)

// Global validator instance
var validate *validator.Validate

// CustomerConfig holds the configuration for the customer service
type CustomerConfig struct {
	Port            string
	StripeSecretKey string
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration

	// Rate Limiter Configuration
	GlobalRateLimit rate.Limit
	GlobalRateBurst int
	IPRateLimit     rate.Limit
	IPRateBurst     int
	MaxIPLimiters   int
}

// DefaultConfig returns a default configuration for the customer service
func DefaultConfig() *CustomerConfig {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8001" // Default port for customer service
	}

	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		slog.Warn("STRIPE_SECRET_KEY is not set. Customer service may not function correctly.")
	}

	return &CustomerConfig{
		Port:            port,
		StripeSecretKey: stripeKey,
		RequestTimeout:  10 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		GlobalRateLimit: rate.Limit(50),
		GlobalRateBurst: 100,
		IPRateLimit:     rate.Limit(10),
		IPRateBurst:     20,
		MaxIPLimiters:   5000,
	}
}

func (c *CustomerConfig) Validate() error {
	if c.Port == "" {
		return errors.New("Port must be set")
	}
	if c.StripeSecretKey == "" {
		return errors.New("StripeSecretKey must be set")
	}
	if c.RequestTimeout <= 0 || c.ShutdownTimeout <= 0 {
		return errors.New("Timeout parameters must be positive")
	}
	if c.GlobalRateLimit <= 0 || c.GlobalRateBurst <= 0 || c.IPRateLimit <= 0 || c.IPRateBurst <= 0 || c.MaxIPLimiters <= 0 {
		return errors.New("Rate limit parameters must be positive")
	}
	return nil
}

type CustomerService struct {
	customers *stripeclient.Client
	config    *CustomerConfig
}

// Local request struct, distinct from contracts if needed for specific validation/binding
type CreateCustomerRequest struct {
	Email    string            `json:"email" validate:"required,email"`
	Name     string            `json:"name" validate:"required,min=2,max=100"`
	Metadata map[string]string `json:"metadata" validate:"dive,keys,min=1,endkeys,min=1"`
}

func NewCustomerService(cfg *CustomerConfig) *CustomerService {
	stripeAPIClient := stripeclient.New(cfg.StripeSecretKey)
	return &CustomerService{
		customers: stripeAPIClient,
		config:    cfg,
	}
}

// Middleware is now provided by the shared middleware package
// --- Middleware End ---

func (s *CustomerService) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.config.RequestTimeout)
	defer cancel()
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 1*1024*1024) // 1MB limit for now

	var req CreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesError *http.MaxBytesError
		var syntaxError *json.SyntaxError
		if errors.As(err, &maxBytesError) {
			slog.WarnContext(ctx, "Request body too large for create customer", "error", err, "limit_bytes", maxBytesError.Limit, "remote_addr", r.RemoteAddr)
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		} else if errors.As(err, &syntaxError) {
			slog.WarnContext(ctx, "Malformed JSON request body for create customer", "error", err, "offset", syntaxError.Offset, "remote_addr", r.RemoteAddr)
			http.Error(w, "Malformed JSON: "+err.Error(), http.StatusBadRequest)
		} else {
			slog.WarnContext(ctx, "Invalid request body for create customer", "error", err, "remote_addr", r.RemoteAddr)
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		}
		return
	}

	if validationErrs := validateStruct(req); validationErrs != nil {
		slog.WarnContext(ctx, "Invalid create customer request data", "errors", validationErrs, "email", req.Email, "name", req.Name)
		http.Error(w, strings.Join(validationErrs, ", "), http.StatusBadRequest)
		return
	}

	stripeCustomer, err := s.customers.CreateCustomer(ctx, req.Email, req.Name, req.Metadata)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create Stripe customer", "error", err, "email", req.Email)
		http.Error(w, "Failed to create customer", http.StatusInternalServerError)
		return
	}
	slog.InfoContext(ctx, "Customer created successfully", "customer_id", stripeCustomer.ID, "email", stripeCustomer.Email)

	customer := &contracts.Customer{
		ID:        stripeCustomer.ID,
		Email:     stripeCustomer.Email,
		Name:      stripeCustomer.Name,
		StripeID:  stripeCustomer.ID,
		Metadata:  stripeCustomer.Metadata,
		CreatedAt: time.Unix(stripeCustomer.Created, 0),
		UpdatedAt: time.Unix(stripeCustomer.Created, 0),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(customer)
}

func validateStruct(s interface{}) []string {
	var errors []string
	err := validate.Struct(s)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			// Customize error messages if needed
			errors = append(errors, fmt.Sprintf("Field '%s' failed on the '%s' tag", err.Field(), err.Tag()))
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}

func (s *CustomerService) GetCustomer(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.config.RequestTimeout)
	defer cancel()

	vars := mux.Vars(r)
	customerID := strings.TrimSpace(vars["id"])
	if customerID == "" {
		slog.WarnContext(ctx, "Customer ID is required for GetCustomer")
		http.Error(w, "Customer ID is required", http.StatusBadRequest)
		return
	}

	stripeCustomer, err := s.customers.GetCustomer(ctx, customerID)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to retrieve Stripe customer", "error", err, "customer_id", customerID)
		http.Error(w, "Failed to retrieve customer", http.StatusInternalServerError)
		return
	}
	slog.InfoContext(ctx, "Customer retrieved successfully", "customer_id", stripeCustomer.ID, "email", stripeCustomer.Email)

	customer := &contracts.Customer{
		ID:        stripeCustomer.ID,
		Email:     stripeCustomer.Email,
		Name:      stripeCustomer.Name,
		StripeID:  stripeCustomer.ID,
		Metadata:  stripeCustomer.Metadata,
		CreatedAt: time.Unix(stripeCustomer.Created, 0),
		UpdatedAt: time.Unix(stripeCustomer.Created, 0),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customer)
}

func (s *CustomerService) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"service":   "customer-service",
		"timestamp": time.Now().Format(time.RFC3339Nano),
	})
}

func setupLogger() *slog.Logger {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	})
	logger := slog.New(jsonHandler)
	slog.SetDefault(logger)
	return logger
}

func setupConfig() *CustomerConfig {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid customer service configuration", "error", err)
		os.Exit(1)
	}
	slog.Info("Customer service configuration loaded", "port", cfg.Port)
	return cfg
}

func setupService(cfg *CustomerConfig) *CustomerService {
	return NewCustomerService(cfg)
}

func setupRouter(service *CustomerService, logger *slog.Logger, cfg *CustomerConfig) *mux.Router {
	rateLimiterCfg := &middleware.RateLimiterConfig{
		GlobalLimit:   cfg.GlobalRateLimit,
		GlobalBurst:   cfg.GlobalRateBurst,
		IPLimit:       cfg.IPRateLimit,
		IPBurst:       cfg.IPRateBurst,
		MaxIPLimiters: cfg.MaxIPLimiters,
		Logger:        logger,
	}
	sharedRateLimiter := middleware.NewRateLimiter(rateLimiterCfg)

	r := mux.NewRouter()
	apiRouter := r.PathPrefix("/api/v1/customers").Subrouter()
	apiRouter.Use(middleware.SecurityHeadersMiddleware)
	apiRouter.Use(sharedRateLimiter.Middleware)
	apiRouter.HandleFunc("", service.CreateCustomer).Methods("POST")
	apiRouter.HandleFunc("/{id}", service.GetCustomer).Methods("GET")

	r.HandleFunc("/health", service.healthCheck).Methods("GET")
	return r
}

func runServer(server *http.Server, shutdownTimeout time.Duration) error {
	go func() {
		slog.Info("🚀 Customer service starting", "port", server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("Customer service server error", "error", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("⏹️  Shutting down customer service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Customer service HTTP server shutdown error", "error", err)
		return err
	}

	slog.Info("✅ Customer service shutdown complete")
	return nil
}

func main() {
	validate = validator.New()
	logger := setupLogger()
	cfg := setupConfig()
	service := setupService(cfg)
	router := setupRouter(service, logger, cfg)
	server := &http.Server{Addr: cfg.Port, Handler: router}
	runServer(server, cfg.ShutdownTimeout)
}
