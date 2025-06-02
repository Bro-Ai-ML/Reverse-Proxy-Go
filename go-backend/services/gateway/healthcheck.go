package main

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type HealthChecker struct {
	backends map[string]*Backend
	mu       sync.RWMutex
}

type Backend struct {
	URL       string
	Healthy   bool
	LastCheck time.Time
}

func NewHealthChecker(backends map[string]string) *HealthChecker {
	b := make(map[string]*Backend)
	for name, url := range backends {
		b[name] = &Backend{URL: url, Healthy: false}
	}
	return &HealthChecker{backends: b}
}

func (h *HealthChecker) Start(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			h.checkAll()
		}
	}()
}

func (h *HealthChecker) checkAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for name, backend := range h.backends {
		go func(n string, b *Backend) {
			client := http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get(b.URL + "/health")
			b.Healthy = err == nil && resp != nil && resp.StatusCode == 200
			b.LastCheck = time.Now()
			if !b.Healthy {
				slog.Error("backend unhealthy", "name", n, "url", b.URL)
			}
		}(name, backend)
	}
}

func (h *HealthChecker) IsHealthy(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	b, ok := h.backends[name]
	return ok && b.Healthy
}
