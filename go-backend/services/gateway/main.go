package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gateway/config"
	"gateway/internal/auth"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Printf("WARNING: invalid duration %q for %s, using default %s", v, key, def)
	}
	return def
}

// loadConfig builds the gateway configuration from environment variables.
// (The previous version hardcoded every value in main.)
func loadConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Port = envOr("PORT", "8080")
	cfg.Server.ReadTimeout = envDuration("READ_TIMEOUT", 10*time.Second)
	cfg.Server.WriteTimeout = envDuration("WRITE_TIMEOUT", 30*time.Second)
	cfg.Server.IdleTimeout = envDuration("IDLE_TIMEOUT", 60*time.Second)
	cfg.Auth.JWTPrivateKeyPath = envOr("JWT_PRIVATE_KEY_PATH", "./secrets/keys/private.pem")
	cfg.Auth.JWTPublicKeyPath = envOr("JWT_PUBLIC_KEY_PATH", "./secrets/keys/public.pem")
	cfg.Auth.TokenDuration = envDuration("TOKEN_DURATION", 15*time.Minute)
	cfg.Auth.RefreshDuration = envDuration("REFRESH_DURATION", 7*24*time.Hour)
	return cfg
}

// parseUpstreams parses UPSTREAMS="name=http://host:port,name2=http://host2:port".
func parseUpstreams(raw string) (map[string]string, error) {
	upstreams := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return upstreams, nil
	}
	for _, part := range strings.Split(raw, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 || kv[0] == "" {
			return nil, fmt.Errorf("invalid UPSTREAMS entry %q, expected name=url", part)
		}
		if _, err := url.Parse(kv[1]); err != nil {
			return nil, fmt.Errorf("invalid upstream URL for %s: %w", kv[0], err)
		}
		upstreams[kv[0]] = kv[1]
	}
	return upstreams, nil
}

func buildProxies(upstreams map[string]string) (map[string]*httputil.ReverseProxy, error) {
	proxies := make(map[string]*httputil.ReverseProxy, len(upstreams))
	for name, raw := range upstreams {
		target, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		p := httputil.NewSingleHostReverseProxy(target)
		p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error for upstream %s: %v", name, err)
			if r.Context().Err() != nil {
				return // client went away; nothing to write
			}
			auth.RespondWithJSON(w, http.StatusBadGateway, map[string]string{
				"error":    "bad gateway",
				"upstream": name,
			})
		}
		proxies[name] = p
	}
	return proxies, nil
}

func main() {
	cfg := loadConfig()

	// JWT keys: fail fast with an actionable message instead of ignoring the
	// read errors (previous behavior started with empty keys and died later).
	privKey, err := os.ReadFile(cfg.Auth.JWTPrivateKeyPath)
	if err != nil {
		log.Fatalf("Failed to read JWT private key at %s: %v\nGenerate a keypair, e.g.:\n  mkdir -p secrets/keys && openssl genrsa -out secrets/keys/private.pem 2048 && openssl rsa -in secrets/keys/private.pem -pubout -out secrets/keys/public.pem", cfg.Auth.JWTPrivateKeyPath, err)
	}
	pubKey, err := os.ReadFile(cfg.Auth.JWTPublicKeyPath)
	if err != nil {
		log.Fatalf("Failed to read JWT public key at %s: %v", cfg.Auth.JWTPublicKeyPath, err)
	}
	jwtManager, err := auth.NewJWTManager(privKey, pubKey, "gateway-service", []string{"gateway-service"})
	if err != nil {
		log.Fatalf("Failed to initialize JWT manager: %v", err)
	}

	refreshStore := auth.NewInMemoryRefreshTokenStore()

	// Periodically sweep expired refresh tokens so the store stays bounded.
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			_ = refreshStore.DeleteExpired()
		}
	}()

	upstreams, err := parseUpstreams(os.Getenv("UPSTREAMS"))
	if err != nil {
		log.Fatalf("Invalid UPSTREAMS configuration: %v", err)
	}
	proxies, err := buildProxies(upstreams)
	if err != nil {
		log.Fatalf("Failed to build reverse proxies: %v", err)
	}

	var health *HealthChecker
	if len(upstreams) > 0 {
		health = NewHealthChecker(upstreams)
		health.Start(envDuration("HEALTH_INTERVAL", 30*time.Second))
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/health"))

	// Public
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		auth.RespondWithJSON(w, 200, map[string]string{"msg": "Welcome"})
	})

	// Auth
	creds := auth.NewEnvDemoCredentialValidator()
	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", jwtManager.LoginHandler(refreshStore, creds, cfg.Auth.TokenDuration, cfg.Auth.RefreshDuration))
		r.Post("/refresh", jwtManager.RefreshHandler(refreshStore, cfg.Auth.TokenDuration, cfg.Auth.RefreshDuration))
		r.Post("/logout", auth.LogoutHandler(refreshStore))
	})

	// Protected API
	r.Route("/api", func(r chi.Router) {
		r.Use(jwtManager.AuthMiddleware)
		r.Route("/user", func(r chi.Router) {
			r.Get("/profile", func(w http.ResponseWriter, r *http.Request) {
				userID, _ := r.Context().Value(auth.UserIDKey).(string)
				auth.RespondWithJSON(w, 200, map[string]interface{}{
					"user_id": userID,
					"name":    "John Doe",
					"email":   "john@example.com",
					"roles":   r.Context().Value(auth.RolesKey),
				})
			})
		})
		// Admin
		r.Route("/admin", func(r chi.Router) {
			r.Use(auth.RequireRole("admin"))
			r.Get("/users", func(w http.ResponseWriter, r *http.Request) {
				auth.RespondWithJSON(w, 200, map[string]interface{}{"users": []string{"user123", "admin456"}})
			})
		})
		// Reports
		r.Route("/reports", func(r chi.Router) {
			r.Use(auth.RequireAnyRole("admin", "reporter"))
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				auth.RespondWithJSON(w, 200, map[string]interface{}{"reports": []string{"r1", "r2"}})
			})
		})

		// Reverse proxy fan-out: /api/v1/{upstream}/... -> configured backend.
		// This is the actual reverse-proxying the repo was named for.
		if len(proxies) > 0 {
			r.Route("/v1", func(r chi.Router) {
				for name, proxy := range proxies {
					name, proxy := name, proxy // capture
					route := fmt.Sprintf("/%s/*", name)
					r.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
						if health != nil && !health.IsHealthy(name) {
							auth.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]string{
								"error":    "upstream unhealthy",
								"upstream": name,
							})
							return
						}
						rest := chi.URLParam(r, "*")
						// Rewrite the path so the upstream sees its own route space.
						r2 := r.Clone(r.Context())
						r2.URL.Path = singleJoiningSlash("/", rest)
						r2.URL.RawPath = ""
						proxy.ServeHTTP(w, r2)
					})
				}
			})
		}
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
		log.Printf("Gateway running on :%s (upstreams: %d)", cfg.Server.Port, len(proxies))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	if health != nil {
		health.Stop()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited properly")
}

func singleJoiningSlash(base, rest string) string {
	if rest == "" {
		return base
	}
	if !strings.HasPrefix(rest, "/") {
		rest = "/" + rest
	}
	return base + strings.TrimPrefix(rest, "/")
}
