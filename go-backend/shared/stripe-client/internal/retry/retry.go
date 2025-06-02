package retry

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"
)

// Func is a function that can be retried.
// It should return an error if it fails and wants to be retried,
// or nil if it succeeds or should not be retried.
type Func func(ctx context.Context) error

// Config holds the configuration for the retry mechanism.
// MaxRetries: The maximum number of times to retry the function (0 means no retries, just one attempt).
// Backoff: The base duration to wait before retrying. Exponential backoff with jitter is used.
// MaxBackoff: The maximum duration to wait before retrying. This caps the exponential growth.
type Config struct {
	MaxRetries int
	Backoff    time.Duration
	MaxBackoff time.Duration
}

// DefaultConfig returns a default configuration for retries.
// It defaults to 3 retries, a 1-second base backoff, and a 30-second max backoff.
func DefaultConfig() Config {
	return Config{
		MaxRetries: 3,
		Backoff:    time.Second,
		MaxBackoff: 30 * time.Second,
	}
}

// Do executes the given function `fn` with retry logic based on the provided `config`.
// It honors context cancellation. If the `ctx` is cancelled, `Do` will stop retrying
// and return the context error.
// If `fn` succeeds (returns nil), `Do` returns nil immediately.
// If `fn` fails after all retries, `Do` returns an error that includes the last error from `fn`
// and the number of retries performed.
func Do(ctx context.Context, config Config, fn Func) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check for context cancellation before attempting the function call.
		if ctx.Err() != nil {
			log.Printf("Retry context cancelled before attempt %d: %v", attempt, ctx.Err())
			return ctx.Err()
		}

		err := fn(ctx) // Pass the potentially cancellable context to the function.
		if err == nil {
			return nil // Success
		}
		lastErr = err // Store the last error encountered.

		if attempt == config.MaxRetries {
			break // Max retries reached, loop will terminate and return lastErr.
		}

		backoffDuration := calculateBackoff(config, attempt)
		log.Printf("Retrying function call after %v (attempt %d/%d), last error: %v",
			backoffDuration, attempt+1, config.MaxRetries, err)

		// Wait for backoff duration or context cancellation.
		select {
		case <-time.After(backoffDuration):
			// Continue to the next attempt.
		case <-ctx.Done():
			log.Printf("Retry context cancelled during backoff (attempt %d): %v", attempt, ctx.Err())
			return ctx.Err() // Context cancelled during backoff.
		}
	}
	// All retries failed.
	return fmt.Errorf("function call failed after %d retries: %w", config.MaxRetries, lastErr)
}

// calculateBackoff calculates the backoff duration for the current attempt
// using exponential backoff (Backoff * 2^attempt) with jitter.
// Jitter is a random percentage (0-50%) of the base backoff time, added to the exponential backoff.
// The duration is capped by config.MaxBackoff.
// `attempt` is 0-indexed.
func calculateBackoff(config Config, attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0 // Ensure attempt is not negative.
	}
	// Exponential backoff: Backoff * 2^attempt
	backoffVal := float64(config.Backoff) * math.Pow(2, float64(attempt))

	// Add jitter: random value between 0 and Backoff * 0.5
	// Seeding rand here is for simplicity in this utility. For high-frequency, concurrent use,
	// a shared, locked rand.Rand or per-goroutine rand.Rand might be preferred.
	rand.Seed(time.Now().UnixNano())
	jitter := rand.Float64() * float64(config.Backoff) * 0.5
	duration := time.Duration(backoffVal + jitter)

	if duration > config.MaxBackoff {
		duration = config.MaxBackoff
	}
	// Safety check for negative duration, though unlikely with positive inputs.
	if duration < 0 {
		duration = config.Backoff // Fallback to base backoff.
	}
	return duration
}
