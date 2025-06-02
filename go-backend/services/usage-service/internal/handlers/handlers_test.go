package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stripe-ecosystem/services/usage-service/internal/config"
	"github.com/stripe-ecosystem/shared/contracts"
)

type mockPool struct {
	submitResult bool
	processed    uint64
	errors       uint64
}

func (m *mockPool) Submit(event contracts.UsageEvent) bool {
	return m.submitResult
}

func (m *mockPool) Stats() (processed, errors uint64) {
	return m.processed, m.errors
}

func TestTrackUsage(t *testing.T) {
	tests := []struct {
		name           string
		payload        interface{}
		submitResult   bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "valid usage event",
			payload: contracts.UsageEvent{
				CustomerID:         "cust_123",
				SubscriptionItemID: "si_123",
				Quantity:           100,
			},
			submitResult:   true,
			expectedStatus: http.StatusOK,
			expectedBody:   "accepted",
		},
		{
			name:           "invalid JSON",
			payload:        "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing customer ID",
			payload: contracts.UsageEvent{
				SubscriptionItemID: "si_123",
				Quantity:           100,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing subscription item ID",
			payload: contracts.UsageEvent{
				CustomerID: "cust_123",
				Quantity:   100,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid quantity",
			payload: contracts.UsageEvent{
				CustomerID:         "cust_123",
				SubscriptionItemID: "si_123",
				Quantity:           -10,
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "service overloaded",
			payload: contracts.UsageEvent{
				CustomerID:         "cust_123",
				SubscriptionItemID: "si_123",
				Quantity:           100,
			},
			submitResult:   false,
			expectedStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cfg := &config.ServiceConfig{RequestTimeout: 5 * time.Second, MaxRequestSizeMB: 10}
			pool := &mockPool{submitResult: tt.submitResult}
			handler := New(pool, cfg)

			// Create request
			var body bytes.Buffer
			json.NewEncoder(&body).Encode(tt.payload)
			req := httptest.NewRequest("POST", "/usage", &body)
			w := httptest.NewRecorder()

			// Execute
			handler.TrackUsage(w, req)

			// Verify
			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" {
				responseBody := w.Body.String()
				if !bytes.Contains([]byte(responseBody), []byte(tt.expectedBody)) {
					t.Errorf("expected body to contain %q, got %q", tt.expectedBody, responseBody)
				}
			}
		})
	}
}

func TestGetStats(t *testing.T) {
	// Setup
	cfg := &config.ServiceConfig{}
	pool := &mockPool{processed: 1000, errors: 50}
	handler := New(pool, cfg)

	// Create request
	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.GetStats(w, req)

	// Verify
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if stats["processed_events"] != float64(1000) {
		t.Errorf("expected processed_events 1000, got %v", stats["processed_events"])
	}

	if stats["error_count"] != float64(50) {
		t.Errorf("expected error_count 50, got %v", stats["error_count"])
	}

	expectedSuccessRate := float64(1000) / float64(1050) * 100.0
	if stats["success_rate"] != expectedSuccessRate {
		t.Errorf("expected success_rate %f, got %v", expectedSuccessRate, stats["success_rate"])
	}
}

func TestHealth(t *testing.T) {
	// Setup
	cfg := &config.ServiceConfig{}
	pool := &mockPool{}
	handler := New(pool, cfg)

	// Create request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Execute
	handler.Health(w, req)

	// Verify
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var health map[string]string
	if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if health["status"] != "healthy" {
		t.Errorf("expected status healthy, got %s", health["status"])
	}
}

func TestValidateUsageEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   contracts.UsageEvent
		wantErr bool
	}{
		{
			name: "valid event",
			event: contracts.UsageEvent{
				CustomerID:         "cust_123",
				SubscriptionItemID: "si_123",
				Quantity:           100,
			},
			wantErr: false,
		},
		{
			name: "missing customer ID",
			event: contracts.UsageEvent{
				SubscriptionItemID: "si_123",
				Quantity:           100,
			},
			wantErr: true,
		},
		{
			name: "missing subscription item ID",
			event: contracts.UsageEvent{
				CustomerID: "cust_123",
				Quantity:   100,
			},
			wantErr: true,
		},
		{
			name: "zero quantity",
			event: contracts.UsageEvent{
				CustomerID:         "cust_123",
				SubscriptionItemID: "si_123",
				Quantity:           0,
			},
			wantErr: true,
		},
		{
			name: "negative quantity",
			event: contracts.UsageEvent{
				CustomerID:         "cust_123",
				SubscriptionItemID: "si_123",
				Quantity:           -10,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUsageEvent(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateUsageEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCalculateSuccessRate(t *testing.T) {
	tests := []struct {
		name      string
		processed uint64
		errors    uint64
		expected  float64
	}{
		{"no events", 0, 0, 100.0},
		{"all success", 100, 0, 100.0},
		{"all errors", 0, 100, 0.0},
		{"mixed", 900, 100, 90.0},
		{"low success", 1, 99, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSuccessRate(tt.processed, tt.errors)
			if result != tt.expected {
				t.Errorf("calculateSuccessRate(%d, %d) = %f, want %f",
					tt.processed, tt.errors, result, tt.expected)
			}
		})
	}
}
