package batch

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager orchestrates the batching of metrics.
// It collects metrics, groups them by an ID (e.g., subscription item ID),
// and flushes them either when a batch reaches a certain size or after a specified interval.
// It uses a Processor interface to handle the actual processing of a complete batch.
// The Manager is safe for concurrent use.
type Manager struct {
	buffer    map[string][]Metric // Stores metrics, keyed by ID. Access is synchronized by mutex.
	mutex     sync.RWMutex
	size      int             // Maximum number of metrics per ID before a batch is flushed.
	interval  time.Duration   // Maximum time to wait before flushing pending batches, regardless of size.
	processor Processor       // Interface responsible for processing a formed batch.
	ctx       context.Context // Context for the manager's lifecycle; cancellation stops the manager.
	done      chan struct{}   // Closed to signal the flushLoop goroutine to terminate.
	wg        sync.WaitGroup  // Used to wait for the flushLoop goroutine to exit gracefully.
}

// Metric represents a single data point to be batched.
// Quantity: The value of the metric (e.g., usage amount).
// Timestamp: When the metric was recorded.
// Action: Describes the metric, e.g., "increment".
type Metric struct {
	Quantity  int64
	Timestamp int64
	Action    string
}

// Batch represents a collection of metrics for a specific ID, ready for processing.
// ID: The identifier for which these metrics are grouped (e.g., subscription_item_id).
// Metrics: The collection of individual Metric data points.
// IdempotencyKey: A unique key for this batch to prevent duplicate processing.
// Timestamp: When the batch was created.
type Batch struct {
	ID             string
	Metrics        []Metric
	IdempotencyKey string
	Timestamp      time.Time
}

// Processor is an interface that defines how a batch of metrics should be processed.
// The Manager calls ProcessBatch when a batch is ready.
type Processor interface {
	ProcessBatch(ctx context.Context, batch Batch) error
}

// NewManager creates a new batch Manager.
// ctx: Parent context for the manager's lifecycle. If cancelled, the manager stops.
// size: The number of metrics per ID to accumulate before flushing.
// interval: The time interval at which to flush all pending batches.
// processor: The Processor implementation that will handle the actual batch processing.
func NewManager(ctx context.Context, size int, interval time.Duration, processor Processor) *Manager {
	return &Manager{
		buffer:    make(map[string][]Metric),
		size:      size,
		interval:  interval,
		processor: processor,
		ctx:       ctx, // Store the parent context
		done:      make(chan struct{}),
	}
}

// Start begins the manager's flushLoop in a separate goroutine.
// This loop is responsible for flushing batches based on the configured interval.
func (m *Manager) Start() {
	m.wg.Add(1)
	go m.flushLoop()
}

// Add appends a metric to the batch for the given ID.
// If adding the metric causes the batch for that ID to reach the configured `size`,
// an asynchronous flush of that specific batch is triggered.
// Returns an error if the manager's context is done (i.e., the manager is shutting down).
func (m *Manager) Add(id string, metric Metric) error {
	// Check if the manager's context is done (e.g., manager is stopped).
	select {
	case <-m.ctx.Done():
		return fmt.Errorf("failed to add metric: manager context done: %w", m.ctx.Err())
	default:
		// Context is not done, proceed to add.
	}

	m.mutex.Lock()
	// Re-check context after acquiring lock, in case of shutdown during wait for lock.
	select {
	case <-m.ctx.Done():
		m.mutex.Unlock()
		return fmt.Errorf("failed to add metric post-lock: manager context done: %w", m.ctx.Err())
	default:
	}

	m.buffer[id] = append(m.buffer[id], metric)

	if len(m.buffer[id]) >= m.size {
		// Prepare metrics for flushing under lock.
		metricsToFlush := make([]Metric, len(m.buffer[id]))
		copy(metricsToFlush, m.buffer[id])
		delete(m.buffer, id) // Clear the flushed metrics from the buffer.
		m.mutex.Unlock()

		// Perform the flush in a goroutine to avoid blocking the Add call.
		// The processing error is logged within doProcessBatch.
		go func() {
			if err := m.doProcessBatch(id, metricsToFlush); err != nil {
				// Log error from async flush; Add call has already returned.
				log.Printf("Error during async flush triggered by Add for ID %s: %v", id, err)
			}
		}()
		return nil // Add itself succeeded in queueing/triggering flush.
	}
	m.mutex.Unlock()
	return nil
}

// doProcessBatch processes a given slice of metrics for a specific ID.
// It constructs a Batch and sends it to the configured Processor.
// This method is called by Add (for size-triggered flushes) and FlushAll (for interval/shutdown flushes).
// Errors from the processor are logged and returned.
func (m *Manager) doProcessBatch(id string, metrics []Metric) error {
	if len(metrics) == 0 {
		return nil // Nothing to process.
	}
	// Create a unique idempotency key for the batch.
	batch := Batch{
		ID:             id,
		Metrics:        metrics,
		IdempotencyKey: fmt.Sprintf("batch-%s-%d", id, time.Now().UnixNano()),
		Timestamp:      time.Now(),
	}

	// Process the batch using the provided processor.
	// The processor is responsible for its own retry logic if needed.
	err := m.processor.ProcessBatch(m.ctx, batch) // Pass manager's context.
	if err != nil {
		log.Printf("Error processing batch for ID %s (idempotency key: %s): %v",
			id, batch.IdempotencyKey, err)
		return err // Propagate error for potential upstream handling.
	}
	log.Printf("Successfully processed batch for ID %s (idempotency key: %s, %d metrics)",
		id, batch.IdempotencyKey, len(metrics))
	return nil
}

// flushLoop is the main goroutine for the Manager.
// It periodically calls FlushAll based on the configured interval.
// It also listens for context cancellation (from parent context) or a signal on m.done
// to trigger a final FlushAll and then terminate.
func (m *Manager) flushLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Printf("Batch manager: interval triggered, flushing all pending batches.")
			m.FlushAll()
		case <-m.ctx.Done(): // If parent context is cancelled
			log.Printf("Batch manager: parent context cancelled, flushing all and shutting down.")
			m.FlushAll()
			return
		case <-m.done: // If Stop() is called
			log.Printf("Batch manager: stop signal received, flushing all and shutting down.")
			m.FlushAll()
			return
		}
	}
}

// FlushAll flushes all currently buffered metrics for all IDs.
// It iterates over a snapshot of the buffer, calling doProcessBatch for each ID.
// This is typically called by the interval timer in flushLoop or during shutdown.
func (m *Manager) FlushAll() {
	m.mutex.Lock()
	// Create a snapshot of the current buffer to process.
	// This avoids holding the lock for the entire duration of processing all batches.
	batchesToProcess := make(map[string][]Metric, len(m.buffer))
	for id, metrics := range m.buffer {
		if len(metrics) > 0 {
			// Deep copy the metrics slice to avoid modification issues if original buffer is accessed.
			copiedMetrics := make([]Metric, len(metrics))
			copy(copiedMetrics, metrics)
			batchesToProcess[id] = copiedMetrics
		}
	}
	m.buffer = make(map[string][]Metric) // Clear the original buffer now that we have a snapshot.
	m.mutex.Unlock()

	// Process the snapshot without holding the main lock.
	log.Printf("FlushAll: Processing %d distinct IDs from buffer snapshot.", len(batchesToProcess))
	for id, metrics := range batchesToProcess {
		if err := m.doProcessBatch(id, metrics); err != nil {
			// Error is already logged in doProcessBatch. Continue flushing other batches.
			log.Printf("FlushAll: Error during processing for ID %s (error logged previously). Will continue with other IDs.", id)
		}
	}
	log.Printf("FlushAll: Finished processing buffer snapshot.")
}

// Stop signals the Manager to flush all pending batches and then shut down its goroutines.
// It blocks until the flushLoop goroutine has completed.
func (m *Manager) Stop() {
	log.Printf("Batch manager: Stop() called, signaling flush and shutdown.")
	close(m.done) // Signal flushLoop to exit.
	m.wg.Wait()   // Wait for flushLoop to complete its final FlushAll and terminate.
	log.Printf("Batch manager: Shutdown complete.")
}
