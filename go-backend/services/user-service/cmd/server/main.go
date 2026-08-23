// Command server is the user-service entrypoint.
//
// It did not exist before: the service shipped handlers, repositories and a
// Makefile but no main package, so nothing could actually run it. This wires
// configuration, PostgreSQL (with migrations), the RGPD handlers and a
// dependency-free HS256 auth middleware.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	handlers "stripe-demo/services/user-service/internal/api/handlers"
	"stripe-demo/services/user-service/internal/config"
	appctx "stripe-demo/services/user-service/internal/context"
	svchandler "stripe-demo/services/user-service/internal/handler"
	"stripe-demo/services/user-service/internal/middleware"
	"stripe-demo/services/user-service/internal/repository/postgres"
	"stripe-demo/services/user-service/internal/service"
	"stripe-demo/services/user-service/pkg/database"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("service", "user-service").Logger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	dbCfg := database.Config{
		Host:     cfg.DB.Host,
		Port:     cfg.DB.Port,
		User:     cfg.DB.User,
		Password: cfg.DB.Password,
		Name:     cfg.DB.Name,
		SSLMode:  cfg.DB.SSLMode,
	}

	db, err := database.NewPostgres(dbCfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	if err := database.RunMigrations(dbCfg); err != nil {
		logger.Fatal().Err(err).Msg("Failed to run migrations")
	}

	sqlxDB := sqlx.NewDb(db.DB, "postgres")

	repo := postgres.NewPostgresRepository(sqlxDB, &logger)
	rgpdSvc := service.NewRGPDSvc(repo)
	rgpdHandlers := handlers.NewRGPDHandlers(rgpdSvc)

	router := mux.NewRouter()
	router.Use(middleware.SecurityHeadersMiddleware)
	router.Use(middleware.LoggingMiddleware(logger))

	svchandler.RegisterRoutes(router) // /health

	authMiddleware := newJWTAuthMiddleware(cfg.JWT.SecretKey)
	rgpdHandlers.RegisterRoutes(router, authMiddleware)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		logger.Info().Str("port", cfg.Server.Port).Msg("user-service starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down user-service...")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Forced shutdown")
	}
	logger.Info().Msg("user-service stopped")
}

// newJWTAuthMiddleware validates HS256 bearer tokens against the configured
// secret and places the authenticated user id in the context using the typed
// helper from internal/context — the same helper the RGPD handlers read.
// (Deliberately dependency-free.)
func newJWTAuthMiddleware(secret string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if len(authHeader) <= len(prefix) || !strings.EqualFold(authHeader[:len(prefix)], prefix) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			userID, ok := verifyHS256(strings.TrimSpace(authHeader[len(prefix):]), []byte(secret))
			if !ok || userID == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := appctx.WithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// verifyHS256 validates an HS256 JWT (constant-time signature comparison,
// exp/nbf checks) and returns the "sub" claim.
func verifyHS256(token string, secret []byte) (string, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil || header.Alg != "HS256" {
		return "", false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", false
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	var claims struct {
		Sub string `json:"sub"`
		Exp *int64 `json:"exp"`
		Nbf *int64 `json:"nbf"`
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return "", false
	}
	now := time.Now().Unix()
	if claims.Exp != nil && now >= *claims.Exp {
		return "", false
	}
	if claims.Nbf != nil && now < *claims.Nbf {
		return "", false
	}
	return claims.Sub, true
}
