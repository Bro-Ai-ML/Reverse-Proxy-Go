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
	
	"your-app/internal/config"
	"your-app/internal/handler"
	"your-app/internal/service"
	"your-app/pkg/database"
)

func main() {
	// Load env
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	// Logger
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "template-service").
		Logger()

	// Config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
	}

	// Database
	db, err := database.NewPostgres(cfg.DB)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Services
	svc := service.New(db, &logger)
	h := handler.New(svc, &logger)

	// Router
	r := mux.NewRouter()
	r.HandleFunc("/health", h.HealthCheck).Methods("GET")
	r.HandleFunc("/api/resource", h.GetResource).Methods("GET")
	r.HandleFunc("/api/resource", h.CreateResource).Methods("POST")

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
