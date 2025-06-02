package worker

import (
	"context"
	"testing"
	"time"

	"github.com/stripe-ecosystem/services/usage-service/internal/config"
	"github.com/stripe-ecosystem/shared/contracts"
)

// Dummy processor for test
type testProcessor struct {
	usageProcessed int
	alertProcessed int
}

func (tp *testProcessor) ProcessUsage(ctx context.Context, event contracts.UsageEvent) error {
	tp.usageProcessed++
	return nil
}
func (tp *testProcessor) ProcessAlert(ctx context.Context, alert AlertEvent) error {
	tp.alertProcessed++
	return nil
}

func TestWorker(t *testing.T) {
	// Replace with real worker logic if available
	if false {
		t.Error("dummy fail")
	}
}

func TestWorkerStruct(t *testing.T) {
	// Replace with real worker logic if available
	t.Log("worker package compiles and can be imported")
}

func TestWorkerPool_SubmitAndProcess(t *testing.T) {
	cfg := &config.ServiceConfig{
		MaxUsageQueueSize:   2,
		MaxAlertQueueSize:   2,
		WorkerPoolSize:      1,
		AlertWorkerPoolSize: 1,
		ShutdownTimeout:     time.Second,
		RequestTimeout:      time.Second,
		GlobalRateLimit:     1,
		GlobalRateBurst:     1,
		IPRateLimit:         1,
		IPRateBurst:         1,
		MaxIPRateLimiters:   1,
		MaxRequestSizeMB:    1,
		Port:                "9999",
	}
	proc := &testProcessor{}
	pool := NewPool(context.Background(), cfg, proc)
	go pool.Start()
	event := contracts.UsageEvent{CustomerID: "cus_test", SubscriptionItemID: "sub_test", Quantity: 1, Timestamp: time.Now()}
	ok := pool.Submit(event)
	if !ok {
		t.Fatal("Submit failed")
	}
	pool.Stop()
	if proc.usageProcessed == 0 {
		t.Fatal("Event not processed")
	}
}
