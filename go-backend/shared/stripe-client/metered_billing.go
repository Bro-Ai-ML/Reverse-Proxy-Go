package stripeclient

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stripe-ecosystem/shared/stripe-client/internal/batch"
	"github.com/stripe-ecosystem/shared/stripe-client/internal/retry"
	"github.com/stripe/stripe-go/v82"
)

// CircuitState represents the state of the circuit breaker.
type CircuitState int

const (
	// StateClosed allows operations to proceed and counts failures.
	StateClosed CircuitState = iota
	// StateOpen rejects operations outright for a cooldown period.
	StateOpen
	// StateHalfOpen allows a single operation to test if the underlying issue is resolved.
	StateHalfOpen
)

// MeteredBillingClient provides a client for Stripe's metered billing functionality.
// It handles batching of usage metrics and ensures that they are reported to Stripe
// efficiently and reliably. It uses an internal batch.Manager for accumulating metrics
// and an internal/retry.Do utility for handling transient errors when communicating with Stripe.
// It also implements a circuit breaker pattern to prevent overwhelming a failing downstream service.
//
// Key features:
//   - Batches usage records by subscription item ID.
//   - Flushes batches based on size or time interval.
//   - Uses Stripe API v82 (specifically, updates subscription item metadata for usage tracking
//     as UsageRecord resources were removed).
//   - Implements retry logic with exponential backoff for Stripe API calls.
//   - Implements a circuit breaker to protect against repeated failures.
//   - Provides thread-safe metric counters for monitoring.
//   - Supports graceful shutdown.
type MeteredBillingClient struct {
	*Client
	// usageBatch     chan UsageBatch // To be removed
	// batchSize      int // To be removed, use batchManager config
	// flushInterval  time.Duration // To be removed, use batchManager config
	// mutex          sync.RWMutex // To be removed, batchManager handles its own concurrency
	// batchBuffer    map[string][]UsageMetric // To be removed
	// done           chan struct{} // To be removed, use batchManager's lifecycle
	wg     sync.WaitGroup // May still be needed for other goroutines if any, or remove if batchManager handles all async
	ctx    context.Context
	cancel context.CancelFunc

	batchManager *batch.Manager // Manages the batching of usage metrics before processing.

	// Metrics and monitoring
	processedCount uint64
	failedCount    uint64
	retryCount     uint64 // This might be redundant if retry utility handles counts, or keep for specific batch retries

	// Configuration
	config *BillingConfig

	// Circuit Breaker State
	cbState               CircuitState
	cbConsecutiveFailures uint32
	cbLastFailureTime     time.Time
	cbMutex               sync.Mutex
}

// Ensure MeteredBillingClient implements batch.Processor
var _ batch.Processor = (*MeteredBillingClient)(nil)

// executeProcessBatch contains the core logic for processing a batch, including retries.
// This is called by ProcessBatch when the circuit breaker allows.
func (mb *MeteredBillingClient) executeProcessBatch(ctx context.Context, b batch.Batch) error {
	totalQuantity := int64(0)
	for _, record := range b.Metrics {
		totalQuantity += record.Quantity
	}

	log.Printf("Executing batch processing: Tracking %d units for subscription item %s (Idempotency: %s, %d records)",
		totalQuantity, b.ID, b.IdempotencyKey, len(b.Metrics))

	retryCfg := retry.DefaultConfig()
	retryCfg.MaxRetries = mb.config.MaxRetries
	// Note: mb.config.RetryBackoff is available if retry.Do needs it directly,
	// or if retry.DefaultConfig() doesn't use a suitable default.

	processingFunc := func(loopCtx context.Context) error {
		// Read current cumulative usage so we INCREMENT instead of overwriting:
		// the previous implementation wrote only this batch's quantity to
		// total_usage, silently discarding every previously reported unit.
		item, err := mb.sc.V1SubscriptionItems.Retrieve(loopCtx, b.ID, &stripe.SubscriptionItemRetrieveParams{})
		if err != nil {
			log.Printf("GetSubscriptionItem attempt for %s (batch: %s): %v", b.ID, b.IdempotencyKey, err)
			return err
		}
		currentUsage := int64(0)
		if item.Metadata != nil {
			if raw, exists := item.Metadata["total_usage"]; exists {
				if _, serr := fmt.Sscanf(raw, "%d", &currentUsage); serr != nil {
					log.Printf("Unparseable total_usage %q for %s, resetting from 0: %v", raw, b.ID, serr)
					currentUsage = 0
				}
			}
		}
		newTotal := currentUsage + totalQuantity

		updateParams := &stripe.SubscriptionItemUpdateParams{
			Metadata: map[string]string{
				"last_usage_update": fmt.Sprintf("%d", time.Now().Unix()),
				"total_usage":       fmt.Sprintf("%d", newTotal),
				"batch_id":          b.IdempotencyKey,
			},
		}
		// Send the batch idempotency key to Stripe so retries of this exact
		// batch cannot be double-counted.
		updateParams.SetIdempotencyKey(b.IdempotencyKey)
		_, err = mb.sc.V1SubscriptionItems.Update(loopCtx, b.ID, updateParams)
		if err != nil {
			log.Printf("UpdateSubscriptionItem attempt for %s (batch: %s): %v", b.ID, b.IdempotencyKey, err)
			return err
		}
		return nil
	}

	err := retry.Do(ctx, retryCfg, processingFunc)
	if err != nil {
		log.Printf("executeProcessBatch failed for %s (batch: %s) after %d retries: %v", b.ID, b.IdempotencyKey, retryCfg.MaxRetries, err)
		log.Printf("🚨 FAILURE (after retries): Could not process batch for %s (Idempotency: %s). Error: %v", b.ID, b.IdempotencyKey, err)
		mb.incrementFailedCount()
		return err
	}

	mb.incrementProcessedCount()
	log.Printf("✅ Successfully processed batch for subscription item %s: %d units (batch: %s)",
		b.ID, totalQuantity, b.IdempotencyKey)

	// Dummy cost calculation, assuming item.Price is populated from Get call
	// This needs to fetch the item again if not already available with price after update, or use Get result
	itemForCost, getItemErr := mb.sc.V1SubscriptionItems.Retrieve(ctx, b.ID, &stripe.SubscriptionItemRetrieveParams{})
	if getItemErr == nil && itemForCost.Price != nil && itemForCost.Price.UnitAmount != 0 {
		estimatedCost := totalQuantity * itemForCost.Price.UnitAmount / 100
		log.Printf("💰 Estimated cost for this batch: %.2f %s (batch: %s)", float64(estimatedCost), string(itemForCost.Price.Currency), b.IdempotencyKey)
	}
	return nil
}

// ProcessBatch is called by the batch.Manager. It wraps executeProcessBatch with circuit breaker logic.
func (mb *MeteredBillingClient) ProcessBatch(ctx context.Context, b batch.Batch) error {
	if !mb.config.CBEnabled {
		return mb.executeProcessBatch(ctx, b)
	}

	mb.cbMutex.Lock()
	currentState := mb.cbState
	if currentState == StateOpen {
		if time.Since(mb.cbLastFailureTime) > mb.config.CBResetTimeout {
			log.Printf("CIRCUIT BREAKER: Transitioning to HalfOpen for %s", b.ID)
			mb.cbState = StateHalfOpen
			currentState = StateHalfOpen
			// Reset consecutive failures for the half-open attempt
			mb.cbConsecutiveFailures = 0
		} else {
			mb.cbMutex.Unlock()
			log.Printf("CIRCUIT BREAKER: Open. Call rejected for batch %s (item %s)", b.IdempotencyKey, b.ID)
			// Note: The batch manager might retry this later if ProcessBatch returns an error.
			// Consider if this should increment global failedCount or a separate CB specific counter.
			// For now, let it be handled as a failure that batch manager might see.
			return fmt.Errorf("circuit breaker is open for item %s", b.ID)
		}
	}
	mb.cbMutex.Unlock() // Release lock before executing, especially if executeProcessBatch is long

	// If StateClosed or StateHalfOpen, execute the operation
	err := mb.executeProcessBatch(ctx, b)

	mb.cbMutex.Lock()
	defer mb.cbMutex.Unlock()

	if err != nil { // Operation failed
		if mb.cbState == StateHalfOpen {
			// Failure in HalfOpen state, transition back to Open
			log.Printf("CIRCUIT BREAKER: Failed in HalfOpen state. Transitioning to Open for %s. Error: %v", b.ID, err)
			mb.cbState = StateOpen
			mb.cbLastFailureTime = time.Now()
		} else if mb.cbState == StateClosed {
			mb.cbConsecutiveFailures++
			if mb.cbConsecutiveFailures >= mb.config.CBFailureThreshold {
				log.Printf("CIRCUIT BREAKER: Failure threshold %d reached. Transitioning to Open for %s. Error: %v", mb.config.CBFailureThreshold, b.ID, err)
				mb.cbState = StateOpen
				mb.cbLastFailureTime = time.Now()
			} else {
				log.Printf("CIRCUIT BREAKER: Recorded consecutive failure %d/%d for %s. Error: %v", mb.cbConsecutiveFailures, mb.config.CBFailureThreshold, b.ID, err)
			}
		}
		return err // Propagate the original error
	}

	// Operation succeeded
	if mb.cbState == StateHalfOpen {
		log.Printf("CIRCUIT BREAKER: Succeeded in HalfOpen state. Transitioning to Closed for %s.", b.ID)
		mb.cbState = StateClosed
		mb.cbConsecutiveFailures = 0
	} else if mb.cbState == StateClosed {
		// If it was already closed and succeeded, reset consecutive failures
		if mb.cbConsecutiveFailures > 0 {
			log.Printf("CIRCUIT BREAKER: Resetting consecutive failures (was %d) due to success in Closed state for %s.", mb.cbConsecutiveFailures, b.ID)
			mb.cbConsecutiveFailures = 0
		}
	}
	return nil
}

// BillingConfig holds configuration parameters for the MeteredBillingClient.
// BatchSize: Number of metrics to accumulate per item before flushing.
// FlushInterval: Maximum time to wait before flushing pending batches.
// MaxRetries: Maximum number of retries for Stripe API calls.
// RetryBackoff: Base duration for exponential backoff on retries (used by retry.Config, effectively).
// MaxQueueSize: Max size of the queue in batch.Manager (indirectly via BatchSize of Manager).
// EnableMetrics: Whether to enable internal metric counters.
// IdempotencyKeyTTL: Time-to-live for idempotency keys (conceptual, actual management by Stripe).
// CBEnabled: Whether the circuit breaker is active.
// CBFailureThreshold: Consecutive failures to open circuit.
// CBResetTimeout: Duration circuit stays open before trying HalfOpen.
type BillingConfig struct {
	BatchSize         int           `json:"batch_size"`
	FlushInterval     time.Duration `json:"flush_interval"`
	MaxRetries        int           `json:"max_retries"`
	RetryBackoff      time.Duration `json:"retry_backoff"`
	MaxQueueSize      int           `json:"max_queue_size"`
	EnableMetrics     bool          `json:"enable_metrics"`
	IdempotencyKeyTTL time.Duration `json:"idempotency_key_ttl"`

	// Circuit Breaker Configuration
	CBEnabled          bool          `json:"cb_enabled"`           // Whether the circuit breaker is active
	CBFailureThreshold uint32        `json:"cb_failure_threshold"` // Consecutive failures to open circuit
	CBResetTimeout     time.Duration `json:"cb_reset_timeout"`     // Duration circuit stays open before trying HalfOpen
}

// DefaultBillingConfig returns a default set of configuration values for the MeteredBillingClient.
func DefaultBillingConfig() *BillingConfig {
	return &BillingConfig{
		BatchSize:         100,
		FlushInterval:     5 * time.Second,
		MaxRetries:        3,
		RetryBackoff:      time.Second,
		MaxQueueSize:      10000,
		EnableMetrics:     true,
		IdempotencyKeyTTL: 24 * time.Hour,
		// Default Circuit Breaker settings
		CBEnabled:          true,
		CBFailureThreshold: 5,                // 5 consecutive failures will open the circuit
		CBResetTimeout:     30 * time.Second, // Circuit stays open for 30s before going to HalfOpen
	}
}

// UsageBatch and UsageMetric are kept for historical context or if direct batching outside manager was ever needed.
// Currently, batch.Manager uses its own batch.Metric struct.
// type UsageBatch struct { ... }
// type UsageMetric struct { ... }

// NewMeteredBilling creates a new MeteredBillingClient with default configuration.
// It initializes and starts the underlying batch.Manager.
func (c *Client) NewMeteredBilling() *MeteredBillingClient {
	return c.NewMeteredBillingWithConfig(DefaultBillingConfig())
}

// NewMeteredBillingWithConfig creates a new MeteredBillingClient with the specified configuration.
// It sets up the client's context for lifecycle management and initializes and starts the batch.Manager,
// passing itself (the MeteredBillingClient) as the batch.Processor. It also initializes circuit breaker state.
func (c *Client) NewMeteredBillingWithConfig(config *BillingConfig) *MeteredBillingClient {
	ctx, cancel := context.WithCancel(context.Background())

	mb := &MeteredBillingClient{
		Client: c,
		ctx:    ctx,
		cancel: cancel,
		config: config,
		// Initialize Circuit Breaker state
		cbState: StateClosed,
		// cbConsecutiveFailures is 0 by default
		// cbLastFailureTime is zero time by default
	}

	// Initialize and start the batch manager
	// The batch.Manager will call mb.ProcessBatch when batches are ready.
	mb.batchManager = batch.NewManager(ctx, config.BatchSize, config.FlushInterval, mb)
	mb.batchManager.Start()

	log.Printf("MeteredBillingClient initialized with Circuit Breaker: Enabled=%v, Threshold=%d, ResetTimeout=%v",
		config.CBEnabled, config.CBFailureThreshold, config.CBResetTimeout)
	return mb
}

// TrackUsage records a single usage metric for a given subscription item.
// It validates the input and then adds the metric to the batch.Manager for asynchronous processing.
// Returns an error if input is invalid or if the batch.Manager fails to queue the metric (e.g., during shutdown).
func (mb *MeteredBillingClient) TrackUsage(ctx context.Context, subscriptionItemID string, quantity int64) error {
	if subscriptionItemID == "" {
		return fmt.Errorf("subscription item ID cannot be empty")
	}
	if quantity <= 0 {
		return fmt.Errorf("quantity must be positive, got: %d", quantity)
	}

	metric := batch.Metric{
		Quantity:  quantity,
		Timestamp: time.Now().Unix(),
		Action:    "increment", // Default action, can be configured if needed
	}

	// Add to the batch manager. The manager handles context cancellation for its internal queue/add.
	if err := mb.batchManager.Add(subscriptionItemID, metric); err != nil {
		// This error typically means the batch manager's context is done (e.g., during shutdown)
		// or an immediate flush failed, which shouldn't happen if Add is just queueing.
		// If batchManager.Add itself can trigger an immediate flush that fails,
		// then more sophisticated error handling or logging might be needed here.
		// For now, assume Add is primarily for queueing.
		log.Printf("Failed to add usage to batch manager for item %s: %v. Possibly shutting down.", subscriptionItemID, err)

		// Fallback: try to process immediately if adding to batch manager queue fails and it's not a context error.
		// This is similar to the original `flushUsageNow` but needs to be adapted.
		// For simplicity now, we'll just return the error from Add.
		// A more robust solution might involve a separate direct flush path if Add fails for non-shutdown reasons.
		return fmt.Errorf("failed to queue usage for item %s: %w", subscriptionItemID, err)
	}

	return nil
}

// processBatches - REMOVED (functionality handled by batch.Manager)

// flushAllPendingBatches - REMOVED (functionality handled by batch.Manager.FlushAll on interval/shutdown)

// flushBatch - REMOVED (logic moved to ProcessBatch, which is called by batch.Manager)

// flushUsageNow - This was a fallback. If batch.Manager.Add fails, that implies manager is shutting down or queue is full.
// The batch.Manager should ideally handle queue full scenarios by blocking or returning specific errors.
// For now, we assume Add handles queueing and its errors are context-related or critical.
// If a direct synchronous flush is still needed as a fallback, it would look like constructing a batch.Batch
// and directly calling mb.ProcessBatch (or a helper that calls it).
// Let's simplify and remove this for now, relying on batchManager.Add() error.
/*
func (mb *MeteredBillingClient) flushUsageNow(ctx context.Context, singleEventUsageBatch UsageBatch) error {
	log.Printf("⚠️ Buffer full or immediate flush requested for item %s. Flushing synchronously.", singleEventUsageBatch.SubscriptionItemID)

	// This needs to be adapted to use the ProcessBatch logic or a direct call
	// Construct a batch.Batch from singleEventUsageBatch
	b := batch.Batch{
		ID: singleEventUsageBatch.SubscriptionItemID,
		Metrics: []batch.Metric{{ // Convert from UsageMetric in singleEventUsageBatch.Records
			Quantity: singleEventUsageBatch.Records[0].Quantity,
			Timestamp: singleEventUsageBatch.Records[0].Timestamp,
			Action: singleEventUsageBatch.Records[0].Action,
		}},
		IdempotencyKey: singleEventUsageBatch.IdempotencyKey,
		Timestamp: singleEventUsageBatch.Timestamp,
	}
	return mb.ProcessBatch(ctx, b) // Directly call the processor method
}
*/

// Shutdown gracefully shuts down the MeteredBillingClient.
// It stops the internal batch.Manager (which will flush any pending batches)
// and then cancels the MeteredBillingClient's own context.
// The `ctx` argument is for the shutdown operation itself, e.g., for a timeout on shutdown.
func (mb *MeteredBillingClient) Shutdown(ctx context.Context) error {
	log.Printf("🛑 Shutting down MeteredBillingClient...")

	// Signal batch manager to stop. This is a blocking call that waits for the manager
	// to flush its pending batches and for its goroutines to exit.
	if mb.batchManager != nil {
		log.Printf("MeteredBillingClient: Stopping batch manager...")
		mb.batchManager.Stop() // This is a blocking call that waits for wg in batchManager
		log.Printf("MeteredBillingClient: Batch manager stopped.")
	}

	// Cancel the MeteredBillingClient's own context. This signals any other potential
	// goroutines managed by MeteredBillingClient (if any were added in the future)
	// to stop. If batchManager was the only user of mb.ctx, this might be redundant
	// after batchManager.Stop(), but it's good practice for a clean shutdown.
	log.Printf("MeteredBillingClient: Cancelling internal context.")
	mb.cancel()

	// Wait for any other goroutines managed directly by MeteredBillingClient (if any).
	// If wg was only for the old processBatches, it might not be needed here.
	// mb.wg.Wait() // Uncomment if MeteredBillingClient still has its own goroutines.

	log.Printf("✅ MeteredBillingClient shutdown complete")
	return nil
}

// GetUsageSummary retrieves a summary of usage for a specific subscription item.
// It fetches the subscription item from Stripe and extracts usage information from its metadata.
// It also includes the client's internal processing metrics.
func (mb *MeteredBillingClient) GetUsageSummary(ctx context.Context, subscriptionItemID string) (*UsageSummary, error) {
	if subscriptionItemID == "" {
		return nil, fmt.Errorf("subscription item ID cannot be empty")
	}
	item, err := mb.sc.V1SubscriptionItems.Retrieve(ctx, subscriptionItemID, &stripe.SubscriptionItemRetrieveParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription item: %w", err)
	}
	currentUsage := int64(0)
	if item.Metadata != nil {
		if usage, exists := item.Metadata["total_usage"]; exists {
			if _, err := fmt.Sscanf(usage, "%d", &currentUsage); err != nil {
				return nil, fmt.Errorf("unparseable total_usage metadata %q: %w", usage, err)
			}
		}
	}
	return &UsageSummary{
		SubscriptionItemID: subscriptionItemID,
		PriceID:            item.Price.ID,
		CurrentPeriodUsage: currentUsage,
		EstimatedCost:      mb.calculateEstimatedCost(ctx, item),
		LastUpdated:        time.Now(),
		ProcessedCount:     mb.getProcessedCount(),
		FailedCount:        mb.getFailedCount(),
		RetryCount:         mb.getRetryCount(),
	}, nil
}

type UsageSummary struct {
	SubscriptionItemID string    `json:"subscription_item_id"`
	PriceID            string    `json:"price_id"`
	CurrentPeriodUsage int64     `json:"current_period_usage"`
	EstimatedCost      int64     `json:"estimated_cost"`
	LastUpdated        time.Time `json:"last_updated"`
	ProcessedCount     uint64    `json:"processed_count"`
	FailedCount        uint64    `json:"failed_count"`
	RetryCount         uint64    `json:"retry_count"`
}

// Metric helper functions (incrementProcessedCount, etc.)
// These need to be made thread-safe using sync/atomic as mb.mutex was removed.

func (mb *MeteredBillingClient) incrementProcessedCount() {
	if mb.config.EnableMetrics {
		atomic.AddUint64(&mb.processedCount, 1)
	}
}

func (mb *MeteredBillingClient) incrementFailedCount() {
	if mb.config.EnableMetrics {
		atomic.AddUint64(&mb.failedCount, 1)
	}
}

func (mb *MeteredBillingClient) incrementRetryCount() {
	// This method is called by the retry utility now, or should be if we want this metric.
	// For now, let's assume retry.Do handles its own logging of retries.
	// If a specific count on mb is needed, retry.Do would need to support a callback.
	// Or, we increment it before calling retry.Do if appropriate.
	// For now, commenting out direct usage if retry utility is the source of truth for retries.
	/*
		if mb.config.EnableMetrics {
			atomic.AddUint64(&mb.retryCount, 1)
		}
	*/
}

func (mb *MeteredBillingClient) getProcessedCount() uint64 {
	if mb.config.EnableMetrics {
		return atomic.LoadUint64(&mb.processedCount)
	}
	return 0
}

func (mb *MeteredBillingClient) getFailedCount() uint64 {
	if mb.config.EnableMetrics {
		return atomic.LoadUint64(&mb.failedCount)
	}
	return 0
}

func (mb *MeteredBillingClient) getRetryCount() uint64 {
	if mb.config.EnableMetrics {
		return atomic.LoadUint64(&mb.retryCount)
	}
	return 0
}

func (mb *MeteredBillingClient) getCurrentPeriodUsage(ctx context.Context, subscriptionItemID string) int64 {
	item, err := mb.sc.V1SubscriptionItems.Retrieve(ctx, subscriptionItemID, &stripe.SubscriptionItemRetrieveParams{})
	if err != nil {
		log.Printf("Error getting subscription item: %v", err)
		return 0
	}
	if item.Metadata != nil {
		if usage, exists := item.Metadata["total_usage"]; exists {
			var currentUsage int64
			if _, err := fmt.Sscanf(usage, "%d", &currentUsage); err != nil {
				log.Printf("Unparseable total_usage metadata %q for item %s: %v", usage, subscriptionItemID, err)
				return 0
			}
			return currentUsage
		}
	}
	return 0
}

func (mb *MeteredBillingClient) calculateEstimatedCost(ctx context.Context, item *stripe.SubscriptionItem) int64 {
	// Calculate based on current usage and price
	usage := mb.getCurrentPeriodUsage(ctx, item.ID) // Pass context

	if item.Price != nil && item.Price.UnitAmount != 0 {
		unitAmount := item.Price.UnitAmount
		return usage * unitAmount / 100 // Convert from cents
	}

	return 0
}
