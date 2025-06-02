package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stripe-demo/services/payment-service/internal/config"
	"stripe-demo/services/payment-service/internal/handlers"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stripe-ecosystem/shared/contracts"
	"github.com/stripe/stripe-go/v82"
)

// MockPaymentProvider is a mock type for the PaymentProvider interface
type MockPaymentProvider struct {
	mock.Mock
}

func (m *MockPaymentProvider) CreatePaymentIntent(ctx context.Context, amount int64, currency string, customerID string, metadata map[string]string) (*stripe.PaymentIntent, error) {
	args := m.Called(ctx, amount, currency, customerID, metadata)
	pi, _ := args.Get(0).(*stripe.PaymentIntent)
	return pi, args.Error(1)
}

func (m *MockPaymentProvider) GetPaymentIntent(ctx context.Context, paymentIntentID string) (*stripe.PaymentIntent, error) {
	args := m.Called(ctx, paymentIntentID)
	pi, _ := args.Get(0).(*stripe.PaymentIntent)
	return pi, args.Error(1)
}

func TestHealthCheck(t *testing.T) {
	cfg := config.DefaultConfig() // Use default config for simplicity
	mockProvider := new(MockPaymentProvider)

	service := handlers.NewService(cfg, mockProvider)

	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(service.HealthCheck)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "handler returned wrong status code")

	expectedBody := map[string]interface{}{
		"status":  "healthy",
		"service": "payment-service",
	}
	var actualBody map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	require.NoError(t, err, "failed to unmarshal response body")

	assert.Equal(t, expectedBody["status"], actualBody["status"])
	assert.Equal(t, expectedBody["service"], actualBody["service"])
	assert.Contains(t, actualBody, "timestamp", "response should contain a timestamp")
}

func TestCreatePayment_Success(t *testing.T) {
	cfg := config.DefaultConfig()
	mockProvider := new(MockPaymentProvider)
	service := handlers.NewService(cfg, mockProvider)

	paymentReq := contracts.PaymentRequest{
		Amount:      1000,
		Currency:    "usd",
		CustomerID:  "cus_test",
		Description: "Test Payment",
		Metadata:    map[string]string{"order_id": "123"},
	}
	payload, _ := json.Marshal(paymentReq)

	req, err := http.NewRequest("POST", "/payments", bytes.NewBuffer(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	mockPI := &stripe.PaymentIntent{
		ID:           "pi_test",
		Amount:       paymentReq.Amount,
		Currency:     stripe.Currency(paymentReq.Currency),
		Status:       stripe.PaymentIntentStatusSucceeded,
		ClientSecret: "pi_test_secret",
		Created:      time.Now().Unix(),
		Customer:     &stripe.Customer{ID: paymentReq.CustomerID},
		Metadata:     paymentReq.Metadata,
	}

	mockProvider.On("CreatePaymentIntent", mock.Anything, paymentReq.Amount, paymentReq.Currency, paymentReq.CustomerID, paymentReq.Metadata).
		Return(mockPI, nil).Once()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(service.CreatePayment)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code, "handler returned wrong status code")

	var respBody contracts.PaymentResponse
	err = json.Unmarshal(rr.Body.Bytes(), &respBody)
	require.NoError(t, err, "failed to unmarshal response body")

	assert.Equal(t, mockPI.ID, respBody.Payment.ID)
	assert.Equal(t, mockPI.ClientSecret, respBody.ClientSecret)
	assert.Equal(t, paymentReq.Amount, respBody.Payment.Amount)
	assert.Equal(t, paymentReq.Currency, respBody.Payment.Currency)
	assert.Equal(t, paymentReq.CustomerID, respBody.Payment.CustomerID)
	assert.Equal(t, string(mockPI.Status), respBody.Payment.Status)

	mockProvider.AssertExpectations(t)
}

func TestCreatePayment_InvalidJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	mockProvider := new(MockPaymentProvider)
	service := handlers.NewService(cfg, mockProvider)

	payload := []byte("{\"amount\":1000, \"currency\":\"usd\", ...invalid_json}")
	req, err := http.NewRequest("POST", "/payments", bytes.NewBuffer(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(service.CreatePayment)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "handler returned wrong status code for invalid JSON")
	assert.Contains(t, rr.Body.String(), "Malformed JSON", "response body should indicate invalid JSON")
}

func TestCreatePayment_BodyTooLarge(t *testing.T) {
	cfg := config.DefaultConfig()
	mockProvider := new(MockPaymentProvider)
	service := handlers.NewService(cfg, mockProvider)

	// Create a payload larger than 1MB (as set in handler)
	// Construct a JSON string that is too large: e.g., {"data": "aaaa..."}
	var sb strings.Builder
	sb.WriteString(`{"data": "`)
	// Max request body size is 1*1024*1024.
	// We need content + overhead (`{"data": "` and `"}`) to exceed this.
	// Overhead is 10 bytes. So, 1MB - 10 bytes + a bit more for the data part.
	for i := 0; i < 1*1024*1024; i++ {
		sb.WriteByte('a')
	}
	sb.WriteString(`"}`)
	largePayload := []byte(sb.String())

	req, err := http.NewRequest("POST", "/payments", bytes.NewBuffer(largePayload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(service.CreatePayment)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code, "handler returned wrong status code for oversized body")
	assert.Contains(t, rr.Body.String(), "Request body too large", "response body should indicate oversized payload")
}

func TestCreatePayment_InvalidRequestData(t *testing.T) {
	tests := []struct {
		name          string
		paymentReq    contracts.PaymentRequest
		expectedError string
	}{
		{
			name: "Negative Amount",
			paymentReq: contracts.PaymentRequest{
				Amount:     -100,
				Currency:   "usd",
				CustomerID: "cus_test",
			},
			expectedError: "Amount must be positive",
		},
		{
			name: "Zero Amount",
			paymentReq: contracts.PaymentRequest{
				Amount:     0,
				Currency:   "usd",
				CustomerID: "cus_test",
			},
			expectedError: "Amount must be positive",
		},
		{
			name: "Missing Currency",
			paymentReq: contracts.PaymentRequest{
				Amount:     1000,
				Currency:   "",
				CustomerID: "cus_test",
			},
			expectedError: "Currency is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			mockProvider := new(MockPaymentProvider)
			service := handlers.NewService(cfg, mockProvider)

			payload, _ := json.Marshal(tc.paymentReq)
			req, err := http.NewRequest("POST", "/payments", bytes.NewBuffer(payload))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(service.CreatePayment)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code, "handler returned wrong status code for invalid data")
			assert.Contains(t, rr.Body.String(), tc.expectedError, "response body should contain specific error message")
		})
	}
}

func TestCreatePayment_StripeAPIError(t *testing.T) {
	cfg := config.DefaultConfig()
	mockProvider := new(MockPaymentProvider)
	service := handlers.NewService(cfg, mockProvider)

	paymentReq := contracts.PaymentRequest{
		Amount:     2000,
		Currency:   "eur",
		CustomerID: "cus_another",
	}
	payload, _ := json.Marshal(paymentReq)

	req, err := http.NewRequest("POST", "/payments", bytes.NewBuffer(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	mockProvider.On("CreatePaymentIntent", mock.Anything, paymentReq.Amount, paymentReq.Currency, paymentReq.CustomerID, paymentReq.Metadata).
		Return(nil, errors.New("stripe API error")).Once()

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(service.CreatePayment)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code, "handler returned wrong status code for Stripe API error")
	assert.Contains(t, rr.Body.String(), "Failed to create payment", "response body should indicate internal server error")
	mockProvider.AssertExpectations(t)
}

func TestGetPayment_Success(t *testing.T) {
	cfg := config.DefaultConfig()
	mockProvider := new(MockPaymentProvider)
	service := handlers.NewService(cfg, mockProvider)

	testPaymentID := "pi_test_get"
	mockPI := &stripe.PaymentIntent{
		ID:       testPaymentID,
		Amount:   5000,
		Currency: stripe.CurrencyUSD,
		Status:   stripe.PaymentIntentStatusSucceeded,
		Created:  time.Now().Unix(),
		Customer: &stripe.Customer{ID: "cus_test_get"},
		Metadata: map[string]string{"order_id": "456"},
	}

	mockProvider.On("GetPaymentIntent", mock.Anything, testPaymentID).Return(mockPI, nil).Once()

	req, err := http.NewRequest("GET", "/payments/"+testPaymentID, nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	// Need a router to extract path variables
	router := mux.NewRouter()
	router.HandleFunc("/payments/{id}", service.GetPayment)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "handler returned wrong status code")

	var respBody contracts.Payment
	err = json.Unmarshal(rr.Body.Bytes(), &respBody)
	require.NoError(t, err, "failed to unmarshal response body")

	assert.Equal(t, mockPI.ID, respBody.ID)
	assert.Equal(t, mockPI.Amount, respBody.Amount)
	assert.Equal(t, string(mockPI.Currency), respBody.Currency)
	assert.Equal(t, string(mockPI.Status), respBody.Status)
	assert.Equal(t, mockPI.Customer.ID, respBody.CustomerID)

	mockProvider.AssertExpectations(t)
}

func TestGetPayment_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	mockProvider := new(MockPaymentProvider)
	service := handlers.NewService(cfg, mockProvider)

	testPaymentID := "pi_nonexistent"

	mockProvider.On("GetPaymentIntent", mock.Anything, testPaymentID).
		Return(nil, errors.New("stripe: No such payment_intent")).Once()

	req, err := http.NewRequest("GET", "/payments/"+testPaymentID, nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router := mux.NewRouter()
	router.HandleFunc("/payments/{id}", service.GetPayment)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code, "handler returned wrong status code for not found PI")
	assert.Contains(t, rr.Body.String(), "Failed to retrieve payment", "response body should indicate internal server error")
	mockProvider.AssertExpectations(t)
}

func TestGetPayment_MissingID(t *testing.T) {
	cfg := config.DefaultConfig()
	mockProvider := new(MockPaymentProvider)
	service := handlers.NewService(cfg, mockProvider)

	// Request to "/payments/" which will result in an empty {id}
	// req, err := http.NewRequest("GET", "/payments/", nil) // Unused variable
	// require.NoError(t, err) // Corresponding require also removed

	rr := httptest.NewRecorder()
	// router := mux.NewRouter() // Unused variable
	// In a real setup, the main router would handle this, but for specific test:
	// we can simulate how Gorilla Mux would behave by setting up a route that would be matched
	// if an ID were present, then calling ServeHTTP with a URL that doesn't provide the ID.
	// However, a direct call to service.GetPayment with a request that simulates this is cleaner.

	// Simulate missing path variable by not setting it in vars
	// This requires directly calling the handler with a request context that doesn't have the ID.
	// The current setup with mux.Vars(r) inside GetPayment makes direct testing of missing ID hard
	// without a full router. Let's assume the router correctly passes it.
	// If the router doesn't match due to a missing segment, it won't call the handler.
	// The test for empty ID inside the handler is if ID is " " or "".

	// Test with an empty ID, assuming mux calls handler even with an empty path param value
	// Note: Realistically, a router might not even route `/payments/` to this handler if the `id` param is expected.
	// This test is more about how the handler behaves if `mux.Vars` returns an empty string for `id`.
	// var err error // Unused
	reqWithPath := httptest.NewRequest("GET", "/payments/", nil) // URL is fine, var is what matters
	// err is implicitly from NewRequest if it were to return one, but it doesn't directly. Let's be explicit if needed.
	// For httptest.NewRequest, an error is not returned. It panics on bad input as seen.
	// The previous panic was due to the bad URL "/payments/ ". The current "/payments/" is valid.
	// So, no explicit error check needed here for NewRequest itself, beyond what require.NoError would do if it returned an error.
	// The require.NoError(t, err) line was a mistake from previous snippet. Removing it.

	reqWithPath = mux.SetURLVars(reqWithPath, map[string]string{"id": ""}) // Set empty id

	handler := http.HandlerFunc(service.GetPayment)
	handler.ServeHTTP(rr, reqWithPath)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "handler returned wrong status code for missing ID")
	assert.Contains(t, rr.Body.String(), "Payment ID is required", "response body should indicate missing ID")
}
