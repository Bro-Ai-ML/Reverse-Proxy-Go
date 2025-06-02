package retry

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestDo_SuccessFirstAttempt(t *testing.T) {
	var callCount int32
	fn := func(ctx context.Context) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}

	err := Do(context.Background(), DefaultConfig(), fn)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("Expected fn to be called once, got %d", callCount)
	}
}

func TestDo_SuccessAfterRetries(t *testing.T) {
	var callCount int32
	failAttempts := 2

	fn := func(ctx context.Context) error {
		currentCall := atomic.AddInt32(&callCount, 1)
		if int(currentCall) <= failAttempts {
			return fmt.Errorf("simulated error attempt %d", currentCall)
		}
		return nil
	}

	config := DefaultConfig()
	config.MaxRetries = 3
	config.Backoff = 1 * time.Millisecond // Speed up test
	config.MaxBackoff = 5 * time.Millisecond

	err := Do(context.Background(), config, fn)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if atomic.LoadInt32(&callCount) != int32(failAttempts+1) {
		t.Errorf("Expected fn to be called %d times, got %d", failAttempts+1, callCount)
	}
}

func TestDo_FailureAllAttempts(t *testing.T) {
	var callCount int32
	fn := func(ctx context.Context) error {
		atomic.AddInt32(&callCount, 1)
		return errors.New("persistent error")
	}

	config := DefaultConfig()
	config.MaxRetries = 2
	config.Backoff = 1 * time.Millisecond
	config.MaxBackoff = 5 * time.Millisecond

	err := Do(context.Background(), config, fn)
	if err == nil {
		t.Errorf("Expected an error, got nil")
	}
	expectedErrMsg := fmt.Sprintf("function call failed after %d retries: persistent error", config.MaxRetries)
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrMsg, err.Error())
	}
	if atomic.LoadInt32(&callCount) != int32(config.MaxRetries+1) {
		t.Errorf("Expected fn to be called %d times, got %d", config.MaxRetries+1, callCount)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	var callCount int32
	fn := func(ctx context.Context) error {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(100 * time.Millisecond) // Simulate work
		return errors.New("should not complete due to cancellation")
	}

	config := DefaultConfig()
	config.Backoff = 1 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond) // Cancel quickly
	defer cancel()

	err := Do(ctx, config, fn)
	if err == nil {
		t.Errorf("Expected a context cancellation error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.DeadlineExceeded or context.Canceled, got %v", err)
	}

	// Depending on timing, fn might be called once before cancellation is caught
	if atomic.LoadInt32(&callCount) > 1 {
		t.Logf("Note: fn was called %d times due to timing with cancellation.", callCount)
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := Config{
		Backoff:    1 * time.Second,
		MaxBackoff: 10 * time.Second,
	}

	tests := []struct {
		attempt     int
		minExpected time.Duration
		maxExpected time.Duration // accounts for jitter up to 0.5 * Backoff
	}{
		{0, 1 * time.Second, 1*time.Second + 500*time.Millisecond},
		{1, 2 * time.Second, 2*time.Second + 500*time.Millisecond},
		{2, 4 * time.Second, 4*time.Second + 500*time.Millisecond},
		{3, 8 * time.Second, 8*time.Second + 500*time.Millisecond},
		{4, 10 * time.Second, 10 * time.Second}, // Capped by MaxBackoff
		{5, 10 * time.Second, 10 * time.Second}, // Stays at MaxBackoff
	}

	for _, tt := range tests {
		d := calculateBackoff(config, tt.attempt)
		if d < tt.minExpected || d > tt.maxExpected {
			t.Errorf("attempt %d: expected backoff between %v and %v, got %v",
				tt.attempt, tt.minExpected, tt.maxExpected, d)
		}
		// Test MaxBackoff capping
		if d > config.MaxBackoff {
			t.Errorf("attempt %d: backoff %v exceeded MaxBackoff %v", tt.attempt, d, config.MaxBackoff)
		}
	}

	// Test negative attempt
	dNeg := calculateBackoff(config, -1)
	if dNeg < config.Backoff || dNeg > config.Backoff+500*time.Millisecond {
		t.Errorf("negative attempt: expected backoff around %v, got %v", config.Backoff, dNeg)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == (Config{}) {
		t.Error("expected non-zero config")
	}
}
