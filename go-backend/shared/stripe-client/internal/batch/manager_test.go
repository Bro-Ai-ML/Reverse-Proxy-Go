package batch

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MockProcessor for testing the batch Manager
type MockProcessor struct {
	processBatchFunc func(ctx context.Context, batch Batch) error
	callCount        int32
	mu               sync.Mutex
	processedBatches []Batch
}

func (mp *MockProcessor) ProcessBatch(ctx context.Context, batch Batch) error {
	atomic.AddInt32(&mp.callCount, 1)
	mp.mu.Lock()
	mp.processedBatches = append(mp.processedBatches, batch)
	mp.mu.Unlock()
	if mp.processBatchFunc != nil {
		return mp.processBatchFunc(ctx, batch)
	}
	return nil
}

func (mp *MockProcessor) GetCallCount() int32 {
	return atomic.LoadInt32(&mp.callCount)
}

func (mp *MockProcessor) GetProcessedBatches() []Batch {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	// Return a copy to avoid race conditions if the caller modifies it
	batchesCopy := make([]Batch, len(mp.processedBatches))
	copy(batchesCopy, mp.processedBatches)
	return batchesCopy
}

func TestManager_AddAndFlushBySize(t *testing.T) {
	mockProc := &MockProcessor{}
	batchSize := 3
	mgr := NewManager(context.Background(), batchSize, 1*time.Hour, mockProc) // Long interval to prevent auto-flush
	mgr.Start()
	defer mgr.Stop()

	for i := 0; i < batchSize-1; i++ {
		err := mgr.Add("item1", Metric{Quantity: int64(i + 1)})
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}
	time.Sleep(50 * time.Millisecond) // Give some time, but not enough for interval flush
	if mockProc.GetCallCount() != 0 {
		t.Errorf("Expected 0 calls to ProcessBatch, got %d", mockProc.GetCallCount())
	}

	// This add should trigger a flush due to size
	err := mgr.Add("item1", Metric{Quantity: int64(batchSize)})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond) // Allow time for async flush to complete

	if mockProc.GetCallCount() != 1 {
		t.Errorf("Expected 1 call to ProcessBatch, got %d", mockProc.GetCallCount())
	}
	bBatches := mockProc.GetProcessedBatches()
	if len(bBatches) != 1 {
		t.Fatalf("Expected 1 processed batch, got %d", len(bBatches))
	}
	if len(bBatches[0].Metrics) != batchSize {
		t.Errorf("Expected batch to have %d metrics, got %d", batchSize, len(bBatches[0].Metrics))
	}
	if bBatches[0].ID != "item1" {
		t.Errorf("Expected batch ID 'item1', got '%s'", bBatches[0].ID)
	}
}

func TestManager_FlushByInterval(t *testing.T) {
	mockProc := &MockProcessor{}
	flushInterval := 100 * time.Millisecond
	mgr := NewManager(context.Background(), 100, flushInterval, mockProc) // Large size to prevent size-flush
	mgr.Start()
	defer mgr.Stop()

	mgr.Add("itemA", Metric{Quantity: 1})
	mgr.Add("itemB", Metric{Quantity: 2})

	if mockProc.GetCallCount() != 0 {
		t.Errorf("Expected 0 calls to ProcessBatch before interval, got %d", mockProc.GetCallCount())
	}

	time.Sleep(flushInterval * 2) // Wait for interval flush to occur

	if mockProc.GetCallCount() != 2 { // Expecting one batch per itemID
		t.Errorf("Expected 2 calls to ProcessBatch after interval (one per itemID), got %d", mockProc.GetCallCount())
	}
	bBatches := mockProc.GetProcessedBatches()
	if len(bBatches) != 2 {
		t.Fatalf("Expected 2 processed batches, got %d", len(bBatches))
	}
	// Check content of batches (order might vary due to map iteration)
	foundItemA := false
	foundItemB := false
	for _, b := range bBatches {
		if b.ID == "itemA" && len(b.Metrics) == 1 && b.Metrics[0].Quantity == 1 {
			foundItemA = true
		}
		if b.ID == "itemB" && len(b.Metrics) == 1 && b.Metrics[0].Quantity == 2 {
			foundItemB = true
		}
	}
	if !foundItemA || !foundItemB {
		t.Errorf("Did not find expected batches for itemA and itemB. A: %t, B: %t", foundItemA, foundItemB)
	}
}

func TestManager_StopFlushesRemaining(t *testing.T) {
	mockProc := &MockProcessor{}
	mgr := NewManager(context.Background(), 10, 1*time.Hour, mockProc)
	mgr.Start()

	mgr.Add("itemX", Metric{Quantity: 100})
	mgr.Add("itemY", Metric{Quantity: 200})

	time.Sleep(50 * time.Millisecond) // Ensure adds are processed by manager's loop if it was channel based
	mgr.Stop()                        // This should trigger FlushAll

	if mockProc.GetCallCount() != 2 {
		t.Errorf("Expected 2 calls to ProcessBatch on Stop, got %d", mockProc.GetCallCount())
	}
	bBatches := mockProc.GetProcessedBatches()
	if len(bBatches) != 2 {
		t.Fatalf("Expected 2 processed batches on Stop, got %d", len(bBatches))
	}
}

func TestManager_AddErrorOnShutdown(t *testing.T) {
	mockProc := &MockProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	mgr := NewManager(ctx, 5, 1*time.Hour, mockProc)
	mgr.Start()

	cancel()   // Cancel the manager's context
	mgr.Stop() // Ensure manager's loop fully stops

	err := mgr.Add("itemZ", Metric{Quantity: 1})
	if err == nil {
		t.Errorf("Expected error when Adding to a stopped manager, got nil")
	} else if !errors.Is(err, context.Canceled) && err.Error() != "failed to add metric: manager context done" { // Check for specific error from Add if not context.Canceled
		t.Errorf("Expected context.Canceled or specific add error, got: %v", err)
	}
}

func TestManager_ProcessorError(t *testing.T) {
	procError := errors.New("processor failed")
	mockProc := &MockProcessor{
		processBatchFunc: func(ctx context.Context, batch Batch) error {
			return procError
		},
	}
	mgr := NewManager(context.Background(), 1, 10*time.Millisecond, mockProc) // Flush quickly
	mgr.Start()
	defer mgr.Stop()

	// This add will trigger an immediate flush
	// The error from ProcessBatch should be logged by the manager, not propagated by Add
	err := mgr.Add("itemErr", Metric{Quantity: 1})
	if err != nil {
		t.Fatalf("Add itself should not fail directly due to processor error: %v", err)
	}

	time.Sleep(50 * time.Millisecond) // Allow flush to happen

	// We can't easily assert logs here, but we ensure Add didn't return the error.
	// In a real scenario, we'd check logs or metrics for processing errors.
	if mockProc.GetCallCount() < 1 {
		t.Errorf("Expected ProcessBatch to be called at least once")
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager(context.Background(), 10, time.Second, nil)
	if m == nil {
		t.Fatal("expected manager, got nil")
	}
}
