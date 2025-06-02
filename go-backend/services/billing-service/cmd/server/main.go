package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"

	"stripe-demo/services/billing-service/internal/config"
	"stripe-demo/services/billing-service/internal/handler"
	"stripe-demo/services/billing-service/internal/service"
	"stripe-demo/services/billing-service/pkg/database"
)

func main() {
	// Load env
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	// Logger
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "billing-service").
		Logger()

	// Config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
	}

	// Database
	dbCfg := database.DBConfig(cfg.DB)
	db, err := database.NewPostgres(dbCfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Run migrations
	if err := database.RunMigrations(dbCfg); err != nil {
		logger.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Services
	svc := service.New(db, &logger, cfg)
	h := handler.New(svc, &logger)

	// Router
	r := mux.NewRouter()

	// Health check
	r.HandleFunc("/health", h.HealthCheck).Methods("GET")

	// API v1 routes with auth middleware
	apiV1 := r.PathPrefix("/api/v1").Subrouter()
	apiV1.Use(h.AuthMiddleware)

	// Billing routes
	billingRouter := apiV1.PathPrefix("/billing").Subrouter()
	{
		// Invoices
		billingRouter.HandleFunc("/invoices", h.ListInvoices).Methods("GET")
		billingRouter.HandleFunc("/invoices/{id}", h.GetInvoice).Methods("GET")
		billingRouter.HandleFunc("/invoices/{id}/pay", h.PayInvoice).Methods("POST")
		billingRouter.HandleFunc("/invoices/{id}/cancel", h.CancelInvoice).Methods("POST")

		// Subscriptions
		billingRouter.HandleFunc("/subscriptions", h.ListSubscriptions).Methods("GET")
		billingRouter.HandleFunc("/subscriptions/{id}", h.GetSubscription).Methods("GET")
		billingRouter.HandleFunc("/subscriptions", h.CreateSubscription).Methods("POST")
		billingRouter.HandleFunc("/subscriptions/{id}/cancel", h.CancelSubscription).Methods("POST")
		billingRouter.HandleFunc("/subscriptions/{id}/update", h.UpdateSubscription).Methods("PUT")

		// Payment Methods
		billingRouter.HandleFunc("/payment-methods", h.ListPaymentMethods).Methods("GET")
		billingRouter.HandleFunc("/payment-methods/{id}", h.GetPaymentMethod).Methods("GET")
		billingRouter.HandleFunc("/payment-methods", h.AddPaymentMethod).Methods("POST")
		billingRouter.HandleFunc("/payment-methods/{id}", h.RemovePaymentMethod).Methods("DELETE")
		billingRouter.HandleFunc("/payment-methods/{id}/default", h.SetDefaultPaymentMethod).Methods("POST")

		// Usage
		billingRouter.HandleFunc("/usage", h.RecordUsage).Methods("POST")
		billingRouter.HandleFunc("/usage/summary", h.GetUsageSummary).Methods("GET")
	}

	// Server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()
	logger.Info().Str("port", cfg.Port).Msg("Server started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown")
	}
	logger.Info().Msg("Server exited")
}
