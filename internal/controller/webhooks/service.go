// Package webhooks provides an asynchronous webhook delivery service.
// Events are published via Emit and delivered to all matching webhook
// subscribers by a pool of background workers.
package webhooks

import (
	"context"
	"log/slog"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// Config holds delivery-service settings.
type Config struct {
	WorkerCount     int
	DeliveryTimeout time.Duration
	MaxRetries      int
}

// Event represents a system event to be delivered to configured webhooks.
type Event struct {
	Type    string         // e.g. "job.completed", "agent.registered"
	Payload map[string]any // event-specific fields
}

// delivery bundles a matched webhook with the event to deliver.
type delivery struct {
	webhook *db.Webhook
	event   Event
}

// Service manages webhook event routing and delivery.
type Service struct {
	store  db.Store
	cfg    Config
	logger *slog.Logger
	queue  chan delivery
}

// New creates a new Service. Call Start to begin background workers.
func New(store db.Store, cfg Config, logger *slog.Logger) *Service {
	workers := cfg.WorkerCount
	if workers <= 0 {
		workers = 4
	}
	return &Service{
		store:  store,
		cfg:    cfg,
		logger: logger,
		queue:  make(chan delivery, workers*10),
	}
}

// Start launches WorkerCount background delivery workers.
// Returns immediately; workers run until ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	workers := s.cfg.WorkerCount
	if workers <= 0 {
		workers = 4
	}
	snd := &sender{
		store:  s.store,
		cfg:    s.cfg,
		logger: s.logger,
	}
	for i := 0; i < workers; i++ {
		go s.worker(ctx, snd)
	}
}

// Emit publishes an event. It queries enabled webhooks subscribed to the
// event type from the DB and enqueues them for delivery. Safe to call
// concurrently. Drops events if the queue is full (warns via logger).
func (s *Service) Emit(ctx context.Context, event Event) {
	hooks, err := s.store.ListWebhooksByEvent(ctx, event.Type)
	if err != nil {
		s.logger.Warn("webhooks: list hooks for event",
			slog.String("event", event.Type),
			slog.String("error", err.Error()),
		)
		return
	}
	for _, wh := range hooks {
		d := delivery{webhook: wh, event: event}
		select {
		case s.queue <- d:
		default:
			s.logger.Warn("webhooks: queue full, dropping delivery",
				slog.String("event", event.Type),
				slog.String("webhook_id", wh.ID),
			)
		}
	}
}

// worker reads deliveries from the queue and sends them.
func (s *Service) worker(ctx context.Context, snd *sender) {
	for {
		select {
		case <-ctx.Done():
			return
		case d := <-s.queue:
			snd.send(ctx, d.webhook, d.event)
		}
	}
}
