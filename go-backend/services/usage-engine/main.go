package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fmt"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
)

// usage-engine: Predictive Usage-Driven Pricing Engine
//
// This microservice ingests API call events, tracks per-customer usage in Redis, and computes a rolling 7-day average.
// When usage crosses thresholds, it can trigger Stripe metered billing or tier upgrades (integration WIP).
//
// Endpoints:
//   POST /event   {"customer_id": string, "timestamp": RFC3339}  # Ingest a usage event
//   GET  /health  # Health check
//
// Redis:
//   - Stores per-customer, per-hour usage as hashes: usage:<customer_id> {<hour>: count}
//   - Rolling average computed over last 7 days (168 hours)
//
// Configuration (env):
//   PORT             - HTTP port (default: 8082)
//   REDIS_URL        - Redis connection URL (default: redis://localhost:6379)
//   STRIPE_API_KEY   - Stripe secret key (default: sk_test_placeholder)
//   SHUTDOWN_TIMEOUT - Graceful shutdown timeout (default: 10s)
//
// See README for more details.

type Config struct {
	Port            string
	ShutdownTimeout time.Duration
	RedisURL        string
	StripeAPIKey    string
}

// RedisUsageStore implements usage tracking using Redis.
type RedisUsageStore struct {
	client *redis.Client
}

func NewRedisUsageStore(redisURL string) (*RedisUsageStore, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}
	return &RedisUsageStore{client: client}, nil
}

// Add increments usage for a customer at a given timestamp (hour granularity).
func (r *RedisUsageStore) Add(customerID string, ts time.Time) error {
	ctx := context.Background()
	hour := ts.Truncate(time.Hour).Format(time.RFC3339)
	key := fmt.Sprintf("usage:%s", customerID)
	return r.client.HIncrBy(ctx, key, hour, 1).Err()
}

// GetRollingAverage returns the rolling average (calls/hour) over the last 7 days for a customer.
func (r *RedisUsageStore) GetRollingAverage(customerID string, now time.Time) (float64, error) {
	ctx := context.Background()
	key := fmt.Sprintf("usage:%s", customerID)
	entries, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	calls := 0
	hours := 0
	start := now.Add(-7 * 24 * time.Hour)
	for hourStr, countStr := range entries {
		hour, err := time.Parse(time.RFC3339, hourStr)
		if err != nil {
			continue // skip malformed
		}
		if hour.After(start) && !hour.After(now) {
			var count int
			fmt.Sscanf(countStr, "%d", &count)
			calls += count
			hours++
		}
	}
	if hours == 0 {
		return 0, nil
	}
	return float64(calls) / float64(hours), nil
}

// Event represents an API call event.
type Event struct {
	CustomerID string    `json:"customer_id"`
	Timestamp  time.Time `json:"timestamp"`
}

func LoadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}
	stripeKey := os.Getenv("STRIPE_API_KEY")
	if stripeKey == "" {
		stripeKey = "sk_test_placeholder"
	}
	shutdownTimeout := 10 * time.Second
	if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			shutdownTimeout = d
		}
	}
	return Config{
		Port:            port,
		ShutdownTimeout: shutdownTimeout,
		RedisURL:        redisURL,
		StripeAPIKey:    stripeKey,
	}
}

func setupLogger() *slog.Logger {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelInfo,
	})
	logger := slog.New(jsonHandler)
	slog.SetDefault(logger)
	return logger
}

func setupRouter(store *RedisUsageStore) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	r.HandleFunc("/event", func(w http.ResponseWriter, r *http.Request) {
		var evt Event
		if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid event"}`))
			return
		}
		if evt.CustomerID == "" || evt.Timestamp.IsZero() {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"missing customer_id or timestamp"}`))
			return
		}
		if err := store.Add(evt.CustomerID, evt.Timestamp); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"failed to store usage"}`))
			return
		}
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("POST")
	return r
}

func startRollingAverageJob(store *RedisUsageStore, interval time.Duration, stopCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				// For demo: list all keys matching usage:*
				ctx := context.Background()
				keys, err := store.client.Keys(ctx, "usage:*").Result()
				if err != nil {
					slog.Error("Failed to list usage keys", "error", err)
					continue
				}
				for _, key := range keys {
					customerID := key[len("usage:"):]
					avg, err := store.GetRollingAverage(customerID, now)
					if err != nil {
						slog.Error("Failed to compute rolling average", "customer_id", customerID, "error", err)
						continue
					}
					slog.Info("Rolling average", "customer_id", customerID, "avg_calls_per_hour", avg)
				}
			case <-stopCh:
				return
			}
		}
	}()
}

func runServer(addr string, handler http.Handler, shutdownTimeout time.Duration) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}
	go func() {
		slog.Info("🚀 Usage engine starting", "address", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("⏹️  Shutting down usage engine...")
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
		return err
	}
	slog.Info("✅ Usage engine shutdown complete")
	return nil
}

func main() {
	logger := setupLogger()
	_ = logger // for future use
	cfg := LoadConfig()
	store, err := NewRedisUsageStore(cfg.RedisURL)
	if err != nil {
		slog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	stopCh := make(chan struct{})
	startRollingAverageJob(store, time.Hour, stopCh)
	router := setupRouter(store)
	runServer(":"+cfg.Port, router, cfg.ShutdownTimeout)
	close(stopCh)
}
