package worker

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stripe-ecosystem/services/usage-service/internal/config"
	"github.com/stripe-ecosystem/shared/contracts"
)

type Pool struct {
	usageEvents    chan contracts.UsageEvent
	alertEvents    chan AlertEvent
	ctx            context.Context
	wg             sync.WaitGroup
	processedCount *uint64
	errorCount     *uint64
	processor      EventProcessor
	cfg            *config.ServiceConfig
}

type AlertEvent struct {
	CustomerID string `json:"customer_id"`
	AlertType  string `json:"alert_type"`
	Message    string `json:"message"`
}

type EventProcessor interface {
	ProcessUsage(ctx context.Context, event contracts.UsageEvent) error
	ProcessAlert(ctx context.Context, alert AlertEvent) error
}

func NewPool(ctx context.Context, cfg *config.ServiceConfig, processor EventProcessor) *Pool {
	return &Pool{
		usageEvents:    make(chan contracts.UsageEvent, cfg.MaxUsageQueueSize),
		alertEvents:    make(chan AlertEvent, cfg.MaxAlertQueueSize),
		ctx:            ctx,
		processor:      processor,
		cfg:            cfg,
		processedCount: new(uint64),
		errorCount:     new(uint64),
	}
}

func (p *Pool) Start() {
	for i := 0; i < p.cfg.WorkerPoolSize; i++ {
		p.wg.Add(1)
		go p.usageWorker(i)
	}

	for i := 0; i < p.cfg.AlertWorkerPoolSize; i++ {
		p.wg.Add(1)
		go p.alertWorker(i)
	}
}

func (p *Pool) usageWorker(id int) {
	defer p.wg.Done()
	slog.Debug("Usage worker started", "worker_id", id)
	for {
		select {
		case event := <-p.usageEvents:
			p.processEvent(id, event)
		case <-p.ctx.Done():
			// Drain whatever is left in the queue instead of dropping it:
			// these events are billing data (the old behavior silently lost
			// every queued event on shutdown/restart).
			drained := 0
			for {
				select {
				case event := <-p.usageEvents:
					p.processEvent(id, event)
					drained++
				default:
					slog.Info("Usage worker shutting down", "worker_id", id, "drained_events", drained)
					return
				}
			}
		}
	}
}

func (p *Pool) processEvent(id int, event contracts.UsageEvent) {
	slog.DebugContext(p.ctx, "Usage worker processing event", "worker_id", id, "customer_id", event.CustomerID, "sub_item_id", event.SubscriptionItemID)
	// Process with a detached context: the pool context is already cancelled
	// during drain, but the event still deserves an honest attempt.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.processor.ProcessUsage(ctx, event); err != nil {
		atomic.AddUint64(p.errorCount, 1)
		slog.ErrorContext(ctx, "Usage worker error processing event", "error", err, "worker_id", id, "customer_id", event.CustomerID, "sub_item_id", event.SubscriptionItemID)
	} else {
		atomic.AddUint64(p.processedCount, 1)
		slog.DebugContext(ctx, "Usage worker successfully processed event", "worker_id", id, "customer_id", event.CustomerID, "sub_item_id", event.SubscriptionItemID)
	}
}

func (p *Pool) alertWorker(id int) {
	defer p.wg.Done()
	slog.Debug("Alert worker started", "worker_id", id)
	for {
		select {
		case alert := <-p.alertEvents:
			slog.InfoContext(p.ctx, "Alert worker processing alert", "worker_id", id, "alert_type", alert.AlertType, "customer_id", alert.CustomerID)
			if err := p.processor.ProcessAlert(p.ctx, alert); err != nil {
				slog.ErrorContext(p.ctx, "Alert worker error processing alert", "error", err, "worker_id", id, "alert_type", alert.AlertType, "customer_id", alert.CustomerID)
			} else {
				slog.DebugContext(p.ctx, "Alert worker successfully processed alert", "worker_id", id, "alert_type", alert.AlertType, "customer_id", alert.CustomerID)
			}
		case <-p.ctx.Done():
			slog.Info("Alert worker shutting down", "worker_id", id)
			return
		}
	}
}

func (p *Pool) Submit(event contracts.UsageEvent) bool {
	select {
	case p.usageEvents <- event:
		slog.DebugContext(p.ctx, "Usage event submitted to pool", "customer_id", event.CustomerID, "sub_item_id", event.SubscriptionItemID)
		return true
	default:
		slog.ErrorContext(p.ctx, "Usage event submission failed (queue full or closed) - CRITICAL", "customer_id", event.CustomerID, "sub_item_id", event.SubscriptionItemID)
		return false
	}
}

func (p *Pool) SubmitAlert(alert AlertEvent) bool {
	select {
	case p.alertEvents <- alert:
		slog.DebugContext(p.ctx, "Alert event submitted to pool", "customer_id", alert.CustomerID, "alert_type", alert.AlertType)
		return true
	default:
		slog.ErrorContext(p.ctx, "Alert event submission failed (queue full or closed) - CRITICAL", "customer_id", alert.CustomerID, "alert_type", alert.AlertType)
		return false
	}
}

func (p *Pool) Stop() {
	p.wg.Wait()
}

func (p *Pool) Stats() (processed, errors uint64) {
	return atomic.LoadUint64(p.processedCount), atomic.LoadUint64(p.errorCount)
}
