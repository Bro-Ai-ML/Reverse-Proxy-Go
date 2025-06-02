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

	"auth-service/internal/config"
	"auth-service/internal/handler"
	"auth-service/internal/service"
	"auth-service/pkg/database"
)

func main() {
	// Load env
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	// Logger
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "auth-service").
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

	// Run migrations
	if err := database.RunMigrations(cfg.DB); err != nil {
		logger.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Services
	svc := service.New(db, &logger, cfg.JWT)
	h := handler.New(svc, &logger, cfg.JWT)

	// Router
	r := mux.NewRouter()
	// Health check
	r.HandleFunc("/health", h.HealthCheck).Methods("GET")
	// Auth routes
	authRouter := r.PathPrefix("/api/v1/auth").Subrouter()
	{
		authRouter.HandleFunc("/register", h.Register).Methods("POST")
		authRouter.HandleFunc("/login", h.Login).Methods("POST")
		authRouter.HandleFunc("/refresh", h.RefreshToken).Methods("POST")
		authRouter.HandleFunc("/verify", h.VerifyToken).Methods("GET")
	}

	// Protected routes
	protectedRouter := r.PathPrefix("/api/v1").Subrouter()
	protectedRouter.Use(h.AuthMiddleware)
	{
		protectedRouter.HandleFunc("/users/me", h.GetCurrentUser).Methods("GET")
		protectedRouter.HandleFunc("/users/me", h.UpdateUser).Methods("PUT")
		protectedRouter.HandleFunc("/users/me/password", h.ChangePassword).Methods("PUT")
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
