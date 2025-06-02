package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gateway/config"
	"gateway/internal/auth"
)

func main() {
	// Config (à remplacer par un vrai loader YAML/env)
	cfg := &config.Config{}
	cfg.Server.Port = "8080"
	cfg.Server.ReadTimeout = 10 * time.Second
	cfg.Server.WriteTimeout = 10 * time.Second
	cfg.Server.IdleTimeout = 60 * time.Second
	cfg.Auth.JWTPrivateKeyPath = "./secrets/keys/private.pem"
	cfg.Auth.JWTPublicKeyPath = "./secrets/keys/public.pem"
	cfg.Auth.TokenDuration = 15 * time.Minute
	cfg.Auth.RefreshDuration = 7 * 24 * time.Hour

	// JWT
	privKey, _ := os.ReadFile(cfg.Auth.JWTPrivateKeyPath)
	pubKey, _ := os.ReadFile(cfg.Auth.JWTPublicKeyPath)
	jwtManager, err := auth.NewJWTManager(privKey, pubKey, "gateway-service", []string{"gateway-service"})
	if err != nil {
		log.Fatalf("Failed to initialize JWT manager: %v", err)
	}

	refreshStore := auth.NewInMemoryRefreshTokenStore()

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.Heartbeat("/health"))

	// Public
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		auth.RespondWithJSON(w, 200, map[string]string{"msg": "Welcome"})
	})

	// Auth
	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", jwtManager.LoginHandler(refreshStore))
		r.Post("/refresh", jwtManager.RefreshHandler(refreshStore))
		r.Post("/logout", auth.LogoutHandler(refreshStore))
	})

	// API protégée
	r.Route("/api", func(r chi.Router) {
		r.Use(jwtManager.AuthMiddleware)
		r.Route("/user", func(r chi.Router) {
			r.Get("/profile", func(w http.ResponseWriter, r *http.Request) {
				userID := r.Context().Value(auth.UserIDKey).(string)
				auth.RespondWithJSON(w, 200, map[string]interface{}{
					"user_id": userID,
					"name":    "John Doe",
					"email":   "john@example.com",
					"roles":   r.Context().Value(auth.RolesKey),
				})
			})
			// Ajoute d'autres handlers user ici
		})
		// Admin
		r.Route("/admin", func(r chi.Router) {
			r.Use(auth.RequireRole("admin"))
			r.Get("/users", func(w http.ResponseWriter, r *http.Request) {
				auth.RespondWithJSON(w, 200, map[string]interface{}{"users": []string{"user123", "admin456"}})
			})
			// Ajoute d'autres handlers admin ici
		})
		// Reports
		r.Route("/reports", func(r chi.Router) {
			r.Use(auth.RequireAnyRole("admin", "reporter"))
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				auth.RespondWithJSON(w, 200, map[string]interface{}{"reports": []string{"r1", "r2"}})
			})
			// Ajoute d'autres handlers reports ici
		})
	})

	// 404/405
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		auth.RespondWithJSON(w, 404, map[string]string{"error": "Not found"})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		auth.RespondWithJSON(w, 405, map[string]string{"error": "Method not allowed"})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		log.Printf("Gateway running on :%s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited properly")
}
