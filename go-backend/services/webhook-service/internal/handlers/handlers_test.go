package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stripe-demo/services/webhook-service/internal/config"
	"stripe-demo/services/webhook-service/internal/handlers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v82/webhook"
)

func TestHealthCheck(t *testing.T) {
	cfg := &config.WebhookConfig{
		// Minimal config needed for health check, or use defaults
		Port:                "8003",
		StripeWebhookSecret: "whsec_testsecret",
		RequestTimeout:      5 * time.Second,
		ShutdownTimeout:     5 * time.Second,
		MaxRequestSizeMB:    2,
	}
	// Initialize with a default config for simplicity in this test
	// More specific config values aren't strictly necessary for HealthCheck
	service := handlers.NewService(cfg)

	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(service.HealthCheck)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "handler returned wrong status code")

	expectedBody := map[string]interface{}{
		"status":  "healthy",
		"service": "webhook-service",
	}
	var actualBody map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	require.NoError(t, err, "failed to unmarshal response body")

	assert.Equal(t, expectedBody["status"], actualBody["status"])
	assert.Equal(t, expectedBody["service"], actualBody["service"])
	assert.Contains(t, actualBody, "timestamp", "response should contain a timestamp")
}

func TestHandleWebhook_Success(t *testing.T) {
	webhookSecret := "whsec_test_valid_secret_for_testing_only"
	cfg := &config.WebhookConfig{
		Port:                "8003",
		StripeWebhookSecret: webhookSecret,
		RequestTimeout:      10 * time.Second, // Increased for potential Stripe processing
		ShutdownTimeout:     5 * time.Second,
		MaxRequestSizeMB:    2,
		GlobalRateLimit:     100,
		GlobalRateBurst:     200,
		IPRateLimit:         50,
		IPRateBurst:         100,
		MaxIPLimiters:       1000,
	}
	service := handlers.NewService(cfg)

	// Sample event payload (Payment Intent Succeeded)
	eventPayload := []byte(`{
	  "id": "evt_1J2n3o4P5q6R7s8T9u0V",
	  "object": "event",
	  "api_version": "2020-08-27",
	  "created": 1625078800,
	  "data": {
	    "object": {
	      "id": "pi_1J2n3o4P5q6R7s8T9u0W",
	      "object": "payment_intent",
	      "amount": 2000,
	      "currency": "usd",
	      "status": "succeeded"
	    }
	  },
	  "livemode": false,
	  "pending_webhooks": 1,
	  "request": {
	    "id": "req_1J2n3o4P5q6R7s8T",
	    "idempotency_key": null
	  },
	  "type": "payment_intent.succeeded"
	}`)

	signedPayload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   eventPayload,
		Secret:    webhookSecret,
		Timestamp: time.Now(),
	})

	req, err := http.NewRequest("POST", "/webhooks/stripe", bytes.NewReader(signedPayload.Payload))
	require.NoError(t, err)
	req.Header.Set("Stripe-Signature", signedPayload.Header)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(service.HandleWebhook)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "handler returned wrong status code for successful webhook")

	var responseBody map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &responseBody)
	require.NoError(t, err, "failed to unmarshal success response body")
	assert.Equal(t, "success", responseBody["status"])
	assert.Equal(t, "evt_1J2n3o4P5q6R7s8T9u0V", responseBody["event_received"], "event_id mismatch in response")

	// TODO: Add assertions for slog output if a mock logger is injected and used.
	// For now, we assume correct processing if status is OK and event ID is acknowledged.
}

func TestHandleWebhook_SignatureFailure(t *testing.T) {
	webhookSecret := "whsec_correct_secret"
	cfg := &config.WebhookConfig{
		Port:                "8003",
		StripeWebhookSecret: webhookSecret,
		RequestTimeout:      5 * time.Second,
		ShutdownTimeout:     5 * time.Second,
		MaxRequestSizeMB:    2,
		// Other config fields as needed, or use DefaultWebhookConfig and override
	}
	service := handlers.NewService(cfg)

	eventPayload := []byte(`{"id": "evt_test", "type": "test.event"}`)

	req, err := http.NewRequest("POST", "/webhooks/stripe", bytes.NewReader(eventPayload))
	require.NoError(t, err)
	// Tamper with the signature or provide a completely invalid one
	req.Header.Set("Stripe-Signature", "t=invalid,v1=invalid_signature_value")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(service.HandleWebhook)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "handler returned wrong status code for signature failure")

	// Optionally, assert the response body contains an error message
	// For example: assert.Contains(t, rr.Body.String(), "Signature verification failed")
}

func TestHandleWebhook_PayloadTooLarge(t *testing.T) {
	webhookSecret := "whsec_test_secret"
	cfg := &config.WebhookConfig{
		Port:                "8003",
		StripeWebhookSecret: webhookSecret,
		RequestTimeout:      5 * time.Second,
		ShutdownTimeout:     5 * time.Second,
		MaxRequestSizeMB:    1, // Set a small limit for testing
	}
	service := handlers.NewService(cfg)

	// Create a payload larger than 1MB
	largePayload := make([]byte, (1*1024*1024)+100) // 1MB + 100 bytes
	for i := range largePayload {
		largePayload[i] = 'a'
	}

	// We still need to sign it, even if it's too large, as signature check might happen after size check
	// or the library might handle it gracefully. The primary test here is the size enforcement.
	signedPayload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   largePayload,
		Secret:    webhookSecret,
		Timestamp: time.Now(),
	})

	req, err := http.NewRequest("POST", "/webhooks/stripe", bytes.NewReader(signedPayload.Payload))
	require.NoError(t, err)
	req.Header.Set("Stripe-Signature", signedPayload.Header)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(service.HandleWebhook)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code, "handler returned wrong status code for oversized payload")
}
