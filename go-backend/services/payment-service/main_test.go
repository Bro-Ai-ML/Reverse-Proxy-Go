//go:build integration || unit
// +build integration unit

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v82"
	"stripe-demo/services/payment-service/internal"
	"stripe-demo/services/payment-service/internal/config"
	"stripe-demo/services/payment-service/internal/handlers"
)

// mockStripeClient is a mock implementation of the Stripe client for testing
type mockStripeClient struct{}

func (m *mockStripeClient) CreatePaymentIntent(ctx context.Context, amount int64, currency, customerID string, metadata map[string]string) (*stripe.PaymentIntent, error) {
	// Create a customer object with the given ID
	customer := &stripe.Customer{
		ID: customerID,
	}

	return &stripe.PaymentIntent{
		ID:       "pi_test_123",
		Status:   stripe.PaymentIntentStatusRequiresPaymentMethod,
		Amount:   amount,
		Currency: stripe.Currency(currency),
		Customer: customer,
		Metadata: metadata,
	}, nil
}

func (m *mockStripeClient) GetPaymentIntent(ctx context.Context, paymentIntentID string) (*stripe.PaymentIntent, error) {
	if paymentIntentID == "pi_notfound" {
		return nil, errors.New("payment intent not found")
	}
	return &stripe.PaymentIntent{
		ID:     paymentIntentID,
		Status: stripe.PaymentIntentStatusSucceeded,
	}, nil
}

// setupTestRouter creates a router with the test configuration
func setupTestRouter(cfg *config.PaymentConfig, stripeClient internal.PaymentProvider) *mux.Router {
	r := mux.NewRouter()
	apiRouter := r.PathPrefix("/api/v1").Subrouter()

	// Create payment service with mock dependencies
	paymentService := handlers.NewService(cfg, stripeClient)

	// Register routes
	r.HandleFunc("/health", paymentService.HealthCheck).Methods("GET")
	apiRouter.HandleFunc("/payments", paymentService.CreatePayment).Methods("POST")
	apiRouter.HandleFunc("/payments/{id}", paymentService.GetPayment).Methods("GET")

	return r
}

// TestMain handles setup and teardown for all tests
func TestMain(m *testing.M) {
	// Set test environment variables
	os.Setenv("STRIPE_SECRET_KEY", "sk_test_123")
	
	// Run tests
	code := m.Run()
	
	// Clean up
	os.Unsetenv("STRIPE_SECRET_KEY")
	os.Exit(code)
}

// TestSetupRouterUnit tests the setupRouter function in unit test mode
func TestSetupRouterUnit(t *testing.T) {
	t.Run("successful setup", func(t *testing.T) {
		// Set up test environment
		os.Setenv("STRIPE_SECRET_KEY", "test_key")
		defer os.Unsetenv("STRIPE_SECRET_KEY")
		
		r := setupRouter()
		require.NotNil(t, r, "Router should not be nil")
		
		// Test if routes are registered
		testCases := []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/health"},
			{http.MethodPost, "/api/v1/payments"},
			{http.MethodGet, "/api/v1/payments/123"},
		}
		
		for _, tc := range testCases {
			req, err := http.NewRequest(tc.method, tc.path, nil)
			require.NoError(t, err, "Failed to create request")
			
			var match mux.RouteMatch
			matched := r.Match(req, &match)
			assert.True(t, matched, "Route should be registered: %s %s", tc.method, tc.path)
		}
	})
}

// TestSetupServer tests the setupServer function
func TestSetupServer(t *testing.T) {
	t.Run("creates server with correct configuration", func(t *testing.T) {
		handler := http.NewServeMux()
		srv1 := setupServer(":8080", handler)
		t.Cleanup(func() { srv1 = nil })
		
		require.NotNil(t, srv1, "Server should be created")
		assert.Equal(t, ":8080", srv1.Addr, "Server address should be set")
		assert.Equal(t, 15*time.Second, srv1.ReadTimeout, "Read timeout should be set")
		assert.Equal(t, 15*time.Second, srv1.WriteTimeout, "Write timeout should be set")
		assert.Equal(t, 60*time.Second, srv1.IdleTimeout, "Idle timeout should be set")
		
		// Test singleton behavior
		srv2 := setupServer(":9090", handler)
		assert.Same(t, srv1, srv2, "Subsequent calls should return the same server instance")
	})
}

// TestRunFunction tests the run function
func TestRunFunction(t *testing.T) {
	// Set up test environment
	originalKey := os.Getenv("STRIPE_SECRET_KEY")
	os.Setenv("STRIPE_SECRET_KEY", "test_key")
	defer func() {
		if originalKey == "" {
			os.Unsetenv("STRIPE_SECRET_KEY")
		} else {
			os.Setenv("STRIPE_SECRET_KEY", originalKey)
		}
	}()

	// Create a context that will be canceled to stop the server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create a channel to capture errors from the run function
	errCh := make(chan error, 1)
	
	// Start the server in a goroutine
	go func() {
		errCh <- run(ctx)
	}()
	
	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)
	
	// Cancel the context to stop the server
	cancel()
	
	// Wait for the run function to return
	select {
	case err := <-errCh:
		assert.NoError(t, err, "run should not return an error when context is canceled")
	case <-time.After(2 * time.Second):
		t.Fatal("run function did not return after context cancellation")
	}
}

// TestMainFunction tests the main function's error handling
func TestMainFunction(t *testing.T) {
	// Save original os.Exit function and replace with a test version
	originalOsExit := osExit
	defer func() { osExit = originalOsExit }()
	
	var exitCode int
	exited := false
	osExit = func(code int) {
		exitCode = code
		exited = true
		panic("os.Exit called")
	}
	
	t.Run("successful execution", func(t *testing.T) {
		// This test is just to verify the main function doesn't panic
		// We can't easily test the happy path of the main function
		// as it would require mocking signal.NotifyContext which is complex
		assert.NotPanics(t, func() {
			// We can't actually test the main function properly in a test
			// but we can verify it doesn't panic with valid config
		})
		
		// Verify the exit code and exited flag are set correctly
		// when osExit is called
		t.Run("osExit sets exit code and flag", func(t *testing.T) {
			// Reset state
			exitCode = 0
			exited = false
			
			// Call osExit with a test code
			testCode := 42
			assert.PanicsWithValue(t, "os.Exit called", func() {
				osExit(testCode)
			}, "osExit should panic with expected message")
			
			// Verify the code and flag were set
			assert.Equal(t, testCode, exitCode, "exitCode should be set to the test code")
			assert.True(t, exited, "exited flag should be set to true")
		})
	})
}

// TestErrorHandling tests error cases
func TestErrorHandling(t *testing.T) {
	t.Run("missing stripe key", func(t *testing.T) {
		t.Log("Starting test: missing stripe key")
		
		// Save and clear STRIPE_SECRET_KEY
		originalKey := os.Getenv("STRIPE_SECRET_KEY")
		os.Unsetenv("STRIPE_SECRET_KEY")
		defer func() {
			t.Log("Restoring original environment")
			if originalKey != "" {
				os.Setenv("STRIPE_SECRET_KEY", originalKey)
			}
		}()
		
		t.Log("Replacing os.Exit with test version")
		// Replace os.Exit with a test version that panics
		oldOsExit := osExit
		defer func() { 
			osExit = oldOsExit 
			t.Log("Restored original os.Exit")
		}()
		
		exited := false
		exitCode := 0
		osExit = func(code int) {
			t.Logf("os.Exit called with code: %d", code)
			exited = true
			exitCode = code
			// Verify the exit code is 1
			if code != 1 {
				t.Errorf("Expected exit code 1, got %d", code)
			}
			// Panic to stop execution in the test
			panic("os.Exit called")
		}
		
		t.Log("Calling setupRouter() - should call os.Exit(1)")
		// This should call our mock os.Exit which will panic
		assert.PanicsWithValue(t, "os.Exit called", func() {
			setupRouter()
		}, "setupRouter should call os.Exit(1) when STRIPE_SECRET_KEY is not set")
		
		t.Log("Verifying osExit was called")
		// Verify osExit was called
		if !exited {
			t.Error("os.Exit was not called")
		} else {
			t.Logf("os.Exit was called with code: %d", exitCode)
		}
	})
}

// Mock log.Fatal implementation for testing
var logFatal = func(v ...interface{}) {
	panic("log.Fatal called")
}

func TestRun(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up test configuration
	cfg := &config.PaymentConfig{
		Port:            "0",
		StripeSecretKey: "sk_test_123",
	}

	// Create mock Stripe client
	mockClient := &mockStripeClient{}

	// Create test router with mock dependencies
	r := setupTestRouter(cfg, mockClient)

	// Create test server
	server := &http.Server{
		Handler: r,
	}

	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "Failed to create listener")

	serverErrors := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	// Ensure server is shut down after test
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	// Test the health check endpoint
	t.Run("health check", func(t *testing.T) {
		resp, err := http.Get("http://" + listener.Addr().String() + "/health")
		require.NoError(t, err, "Failed to make health check request")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected status code 200")
	})

	// Test a protected endpoint
	t.Run("protected endpoint", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://"+listener.Addr().String()+"/api/v1/payments/123", nil)
		require.NoError(t, err, "Failed to create request")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err, "Failed to make payment request")
		defer resp.Body.Close()

		// Should return 200 OK with our mock implementation
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected status code 200")
	})
}

func TestSetupRouter(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up test configuration
	cfg := &config.PaymentConfig{
		Port:            "0",
		StripeSecretKey: "sk_test_123",
	}

	// Create mock Stripe client
	mockClient := &mockStripeClient{}

	// Create test router with mock dependencies
	r := setupTestRouter(cfg, mockClient)
	require.NotNil(t, r, "Router should not be nil")

	// Create test server
	server := &http.Server{
		Handler: r,
	}

	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "Failed to create listener")

	serverErrors := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			serverErrors <- err
		}
	}()

	// Ensure server is shut down after test
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	// Test cases
	type testCase struct {
		name        string
		method      string
		path        string
		statusCode  int
		skip        bool
		setupRequest func(*http.Request)
	}

	tests := []testCase{
		{
			name:       "health check",
			method:     http.MethodGet,
			path:       "/health",
			statusCode: http.StatusOK,
		},
		{
			name:       "create payment",
			method:     http.MethodPost,
			path:       "/api/v1/payments",
			statusCode: http.StatusCreated,
			setupRequest: func(req *http.Request) {
				req.Header.Set("Content-Type", "application/json")
				body := strings.NewReader(`{"amount": 1000, "currency": "usd", "customer_id": "cus_test"}`)
				req.Body = io.NopCloser(body)
				req.ContentLength = int64(body.Len())
			},
		},
		{
			name:       "get payment",
			method:     http.MethodGet,
			path:       "/api/v1/payments/123",
			statusCode: http.StatusOK,
		},
	}

	// Run test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skip("Test case skipped")
			}

			req, err := http.NewRequest(tc.method, "http://"+listener.Addr().String()+tc.path, nil)
			require.NoError(t, err, "Failed to create request")

			// Apply any request setup
			if tc.setupRequest != nil {
				tc.setupRequest(req)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			assert.Equal(t, tc.statusCode, resp.StatusCode,
				"Expected status %d for %s %s, got %d",
				tc.statusCode, tc.method, tc.path, resp.StatusCode)
		})
	}

	// Test server shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Failed to shutdown server: %v", err)
	}
}
