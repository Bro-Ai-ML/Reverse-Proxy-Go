package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"github.com/stripe-ecosystem/shared/middleware"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/stripe-ecosystem/services/usage-service/internal/config"
	"github.com/stripe-ecosystem/services/usage-service/internal/handlers"
	"github.com/stripe-ecosystem/services/usage-service/internal/worker"
	"github.com/stripe-ecosystem/shared/contracts"
	stripeclient "github.com/stripe-ecosystem/shared/stripe-client"
)

// Service handles the usage service functionality

type Service struct {
	stripe         *stripeclient.Client
	meteredBilling *stripeclient.MeteredBillingClient
	pool           *worker.Pool
	handlers       *handlers.Handler
	config         *config.ServiceConfig
	// logger can be added here if needed
}

func (s *Service) ProcessUsage(ctx context.Context, event contracts.UsageEvent) error {
	slog.DebugContext(ctx, "Processing usage event", "subscription_item_id", event.SubscriptionItemID, "quantity", event.Quantity)
	err := s.meteredBilling.TrackUsage(ctx, event.SubscriptionItemID, event.Quantity)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to track usage", "error", err, "subscription_item_id", event.SubscriptionItemID)
	}
	return err
}

func (s *Service) ProcessAlert(ctx context.Context, alert worker.AlertEvent) error {
	// Example: if alert processing could be long, check context
	select {
	case <-ctx.Done():
		slog.InfoContext(ctx, "Alert processing cancelled due to context done.", "alert_type", alert.AlertType, "customer_id", alert.CustomerID)
		return ctx.Err()
	default:
		// Proceed with alert processing
		slog.InfoContext(ctx, "Context-aware 🔔 Alert", "alert_type", alert.AlertType, "message", alert.Message, "customer_id", alert.CustomerID)
		return nil
	}
}

func main() {
	// Configure structured logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	})))

	// Load configuration
	cfg := config.Default()
	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	// Verify required environment variables
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		slog.Error("STRIPE_SECRET_KEY environment variable is required")
		os.Exit(1)
	}

	slog.Info("Starting usage service", "port", cfg.Port, "worker_pool_size", cfg.WorkerPoolSize)

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize service components
	service := &Service{
		stripe:         stripeclient.New(stripeKey),
		meteredBilling: stripeclient.New(stripeKey).NewMeteredBilling(),
		config:         cfg,
	}

	// Initialize worker pool and handlers
	service.pool = worker.NewPool(ctx, cfg, service)
	service.handlers = handlers.New(service.pool, cfg)

	// Start worker pool
	service.pool.Start()
	slog.Info("Worker pool started")

	// Configure rate limiting
	rateLimiter := middleware.NewRateLimiter(&middleware.RateLimiterConfig{
		GlobalLimit:   cfg.GlobalRateLimit,
		GlobalBurst:   cfg.GlobalRateBurst,
		IPLimit:       cfg.IPRateLimit,
		IPBurst:       cfg.IPRateBurst,
		MaxIPLimiters: cfg.MaxIPRateLimiters,
	})

	// Setup HTTP server
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(middleware.SecurityHeadersMiddleware, rateLimiter.Middleware)

	// Register routes
	api.HandleFunc("/usage", service.handlers.TrackUsage).Methods("POST")
	api.HandleFunc("/stats", service.handlers.GetStats).Methods("GET")
	r.HandleFunc("/health", service.handlers.Health).Methods("GET")

	server := &http.Server{
		Addr:    cfg.Port,
		Handler: r,
	}

	// Start HTTP server
	go func() {
		slog.Info("Starting HTTP server", "port", cfg.Port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Begin graceful shutdown
	slog.Info("Shutting down...")
	cancel()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Error during server shutdown", "error", err)
	}

	// Stop worker pool
	if service.pool != nil {
		service.pool.Stop()
		slog.Info("Worker pool stopped")
	}

	// Shutdown metered billing client
	if service.meteredBilling != nil {
		slog.Info("Shutting down metered billing client...")
		mbShutdownCtx, mbCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer mbCancel()
		if err := service.meteredBilling.Shutdown(mbShutdownCtx); err != nil {
			slog.Error("Error shutting down metered billing client", "error", err)
		} else {
			slog.Info("Metered billing client shut down.")
		}
	}

	slog.Info("✅ Shutdown complete")
}
