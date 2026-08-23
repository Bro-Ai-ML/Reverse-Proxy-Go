package main

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type HealthChecker struct {
	backends map[string]*Backend
	client   *http.Client
	mu       sync.RWMutex
	stop     chan struct{}
	stopOnce sync.Once
}

type Backend struct {
	URL       string
	Healthy   bool
	LastCheck time.Time
}

func NewHealthChecker(backends map[string]string) *HealthChecker {
	b := make(map[string]*Backend, len(backends))
	for name, url := range backends {
		b[name] = &Backend{URL: url, Healthy: false}
	}
	return &HealthChecker{
		backends: b,
		client:   &http.Client{Timeout: 2 * time.Second},
		stop:     make(chan struct{}),
	}
}

// Start launches the periodic health checks. It is safe to call once; Stop
// terminates the loop.
func (h *HealthChecker) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		// Probe immediately, then on every tick.
		h.checkAll()
		for {
			select {
			case <-ticker.C:
				h.checkAll()
			case <-h.stop:
				return
			}
		}
	}()
}

func (h *HealthChecker) Stop() {
	h.stopOnce.Do(func() { close(h.stop) })
}

// Names returns the list of configured backend names.
func (h *HealthChecker) Names() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	names := make([]string, 0, len(h.backends))
	for name := range h.backends {
		names = append(names, name)
	}
	return names
}

func (h *HealthChecker) checkAll() {
	h.mu.RLock()
	snapshot := make(map[string]string, len(h.backends))
	for name, backend := range h.backends {
		snapshot[name] = backend.URL
	}
	h.mu.RUnlock()

	var wg sync.WaitGroup
	for name, url := range snapshot {
		wg.Add(1)
		go func(n, u string) {
			defer wg.Done()
			healthy := probe(h.client, u)
			h.mu.Lock()
			if b, ok := h.backends[n]; ok {
				if b.Healthy != healthy {
					if !healthy {
						slog.Error("backend unhealthy", "name", n, "url", u)
					} else {
						slog.Info("backend healthy", "name", n, "url", u)
					}
				}
				b.Healthy = healthy
				b.LastCheck = time.Now()
			}
			h.mu.Unlock()
		}(name, url)
	}
	wg.Wait()
}

func probe(client *http.Client, baseURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close() // previously leaked on every probe
	return resp.StatusCode == http.StatusOK
}

func (h *HealthChecker) IsHealthy(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	b, ok := h.backends[name]
	return ok && b.Healthy
}
