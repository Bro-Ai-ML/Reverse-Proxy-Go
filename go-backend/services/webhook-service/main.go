package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"stripe-demo/services/webhook-service/internal/config"
	"stripe-demo/services/webhook-service/internal/handlers"
	"stripe-demo/shared/middleware"
)

func main() {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	})
	slog.SetDefault(slog.New(jsonHandler))

	cfg := config.DefaultWebhookConfig()
	if err := cfg.Validate(); err != nil {
		slog.Error("Invalid webhook service configuration", "error", err)
		os.Exit(1)
	}
	slog.Info("Webhook service configuration loaded", "port", cfg.Port, "max_request_size_mb", cfg.MaxRequestSizeMB)

	// Verify webhook secret is set
	if cfg.StripeWebhookSecret == "" {
		slog.Warn("STRIPE_WEBHOOK_SECRET is not set. Webhook signature verification will fail!")
	}

	service := handlers.NewService(cfg) // Use handlers.NewService

	rateLimiterCfg := &middleware.RateLimiterConfig{
		GlobalLimit:   cfg.GlobalRateLimit,
		GlobalBurst:   cfg.GlobalRateBurst,
		IPLimit:       cfg.IPRateLimit,
		IPBurst:       cfg.IPRateBurst,
		MaxIPLimiters: cfg.MaxIPLimiters,
		Logger:        slog.Default(),
	}
	sharedRateLimiter := middleware.NewRateLimiter(rateLimiterCfg)

	r := mux.NewRouter()
	webhookRouter := r.PathPrefix("/webhooks").Subrouter()
	webhookRouter.Use(middleware.SecurityHeadersMiddleware)
	webhookRouter.Use(sharedRateLimiter.Middleware)
	webhookRouter.HandleFunc("/stripe", service.HandleWebhook).Methods("POST")

	r.HandleFunc("/health", service.HealthCheck).Methods("GET")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	go func() {
		slog.Info("Starting webhook service", "port", cfg.Port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down webhook service...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Error during server shutdown", "error", err)
	}

	slog.Info("Webhook service stopped")
}
