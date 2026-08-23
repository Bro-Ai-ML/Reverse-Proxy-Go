package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/stripe-ecosystem/shared/middleware"
	stripeclient "github.com/stripe-ecosystem/shared/stripe-client"
	"stripe-demo/services/payment-service/internal/config"
	"stripe-demo/services/payment-service/internal/handlers"
)

// server holds the HTTP server and its configuration (test seam: setupServer
// is a singleton).
var (
	srv     *http.Server
	srvOnce sync.Once
)

// osExit is a variable that holds the function to call to exit the application.
// This allows us to mock os.Exit in tests.
var osExit = os.Exit

// setupRouter creates and configures the HTTP router
func setupRouter() *mux.Router {
	cfg := config.DefaultConfig()

	// Initialize Stripe client
	if cfg.StripeSecretKey == "" {
		slog.Error("STRIPE_SECRET_KEY environment variable is not set")
		osExit(1)
		return nil // This line is unreachable but makes the linter happy
	}

	stripeClient := stripeclient.New(cfg.StripeSecretKey)
	// FIX: the service used to receive a nil config here, so the very first
	// request panicked on s.config.RequestTimeout.
	paymentService := handlers.NewService(cfg, stripeClient)

	rateLimiterCfg := &middleware.RateLimiterConfig{
		GlobalLimit:   cfg.GlobalRateLimit,
		GlobalBurst:   cfg.GlobalRateBurst,
		IPLimit:       cfg.IPRateLimit,
		IPBurst:       cfg.IPBurst,
		MaxIPLimiters: cfg.MaxIPLimiters,
	}
	sharedRateLimiter := middleware.NewRateLimiter(rateLimiterCfg)

	r := mux.NewRouter()
	apiRouter := r.PathPrefix("/api/v1").Subrouter()
	apiRouter.Use(middleware.SecurityHeadersMiddleware)
	apiRouter.Use(sharedRateLimiter.Middleware)
	apiRouter.HandleFunc("/payments", paymentService.CreatePayment).Methods("POST")
	apiRouter.HandleFunc("/payments/{id}", paymentService.GetPayment).Methods("GET")
	r.HandleFunc("/health", paymentService.HealthCheck).Methods("GET")

	return r
}

// setupServer creates and configures the HTTP server (singleton)
func setupServer(addr string, handler http.Handler) *http.Server {
	srvOnce.Do(func() {
		srv = &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}
	})
	return srv
}

// run starts the HTTP server and handles graceful shutdown
func run(ctx context.Context) error {
	// Configure structured logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	})))

	cfg := config.DefaultConfig()
	if err := cfg.Validate(); err != nil {
		return err
	}

	r := setupRouter()

	// FIX: the server used to bind ":0" (a random port), which made the
	// service unreachable behind its compose/Docker port mapping. It now
	// honors the configured PORT.
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("🚀 Payment service starting", "address", server.Addr)

	// Channel to signal server shutdown
	serverErrors := make(chan error, 1)

	// Start the server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	// Handle shutdown signals
	go func() {
		<-ctx.Done()
		slog.Info("⏹️ Shutting down payment service...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Payment service HTTP server shutdown error", "error", err)
			serverErrors <- err
		}
	}()

	// Wait for server to exit or error
	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		return nil
	}
}

func main() {
	// Create a context that cancels on interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}

	slog.Info("✅ Payment service shutdown complete")
}
