// Package webhooks provides an asynchronous webhook delivery service.
// Events are published via Emit and delivered to all matching webhook
// subscribers by a pool of background workers.  When an EmailSender is
// configured, Emit also dispatches email notifications to users whose
// notification preferences request them.
package webhooks

import (
	"context"
	"log/slog"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/notifications"
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
	store    db.Store
	cfg      Config
	logger   *slog.Logger
	queue    chan delivery
	email    *notifications.EmailSender    // nil when email is not configured
	telegram *notifications.TelegramSender // nil when Telegram is not configured
	pushover *notifications.PushoverSender // nil when Pushover is not configured
	ntfy     *notifications.NtfySender     // nil when ntfy is not configured
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

// SetEmailSender attaches an EmailSender. When set, Emit will also send
// email notifications to users whose notification preferences are enabled.
// Call this before Start.
func (s *Service) SetEmailSender(e *notifications.EmailSender) {
	s.email = e
}

// SetTelegramSender attaches a TelegramSender. Call this before Start.
func (s *Service) SetTelegramSender(t *notifications.TelegramSender) {
	s.telegram = t
}

// SetPushoverSender attaches a PushoverSender. Call this before Start.
func (s *Service) SetPushoverSender(p *notifications.PushoverSender) {
	s.pushover = p
}

// SetNtfySender attaches a NtfySender. Call this before Start.
func (s *Service) SetNtfySender(n *notifications.NtfySender) {
	s.ntfy = n
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
// When an EmailSender is configured, email notifications are also dispatched
// asynchronously for users whose preferences request them.
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

	// Dispatch email notifications asynchronously if email is configured.
	if s.email != nil {
		go s.dispatchEmails(ctx, event)
	}

	// Dispatch push channel notifications asynchronously.
	if s.telegram != nil || s.pushover != nil || s.ntfy != nil {
		go s.dispatchPushChannels(ctx, event)
	}
}

// dispatchEmails sends email notifications for event to all users whose
// notification preferences opt them in.
func (s *Service) dispatchEmails(ctx context.Context, event Event) {
	prefs, err := s.store.ListUsersWithEmailNotifications(ctx)
	if err != nil {
		s.logger.Warn("webhooks: list email notification prefs",
			slog.String("event", event.Type),
			slog.String("error", err.Error()),
		)
		return
	}

	for _, p := range prefs {
		if p.EmailAddress == "" {
			continue
		}
		var (
			subject string
			body    string
			renderErr error
		)

		jobID, _ := event.Payload["job_id"].(string)
		sourcePath, _ := event.Payload["source_path"].(string)
		agentName, _ := event.Payload["agent_name"].(string)

		switch event.Type {
		case "job.completed":
			if !p.NotifyOnJobComplete {
				continue
			}
			subject = "Job Completed — EncodeSwarmr"
			body, renderErr = notifications.RenderJobCompleted(jobID, sourcePath)
		case "job.failed":
			if !p.NotifyOnJobFailed {
				continue
			}
			subject = "Job Failed — EncodeSwarmr"
			detail, _ := event.Payload["error"].(string)
			body, renderErr = notifications.RenderJobFailed(jobID, sourcePath, detail)
		case "agent.stale":
			if !p.NotifyOnAgentStale {
				continue
			}
			subject = "Agent Offline — EncodeSwarmr"
			body, renderErr = notifications.RenderAgentStale(agentName)
		default:
			continue
		}

		if renderErr != nil {
			s.logger.Warn("webhooks: render email template",
				slog.String("event", event.Type),
				slog.String("error", renderErr.Error()),
			)
			continue
		}

		if err := s.email.Send(p.EmailAddress, subject, body); err != nil {
			s.logger.Warn("webhooks: send email notification",
				slog.String("event", event.Type),
				slog.String("to", p.EmailAddress),
				slog.String("error", err.Error()),
			)
		}
	}
}

// dispatchPushChannels sends a notification via Telegram, Pushover, and ntfy
// for the given event. Each channel is best-effort: errors are logged but
// never propagate or block event delivery.
func (s *Service) dispatchPushChannels(_ context.Context, event Event) {
	jobID, _ := event.Payload["job_id"].(string)
	sourcePath, _ := event.Payload["source_path"].(string)
	agentName, _ := event.Payload["agent_name"].(string)

	var title, message string
	switch event.Type {
	case "job.completed":
		title = "Job Completed — EncodeSwarmr"
		message = "Job " + jobID + " completed: " + sourcePath
	case "job.failed":
		detail, _ := event.Payload["error"].(string)
		title = "Job Failed — EncodeSwarmr"
		message = "Job " + jobID + " failed: " + sourcePath
		if detail != "" {
			message += "\n" + detail
		}
	case "agent.stale":
		title = "Agent Offline — EncodeSwarmr"
		message = "Agent " + agentName + " has stopped sending heartbeats."
	default:
		return
	}

	if s.telegram != nil {
		if err := s.telegram.Send(title + "\n" + message); err != nil {
			s.logger.Warn("webhooks: telegram notification failed",
				slog.String("event", event.Type),
				slog.String("error", err.Error()),
			)
		}
	}
	if s.pushover != nil {
		if err := s.pushover.Send(title, message); err != nil {
			s.logger.Warn("webhooks: pushover notification failed",
				slog.String("event", event.Type),
				slog.String("error", err.Error()),
			)
		}
	}
	if s.ntfy != nil {
		if err := s.ntfy.Send(title, message); err != nil {
			s.logger.Warn("webhooks: ntfy notification failed",
				slog.String("event", event.Type),
				slog.String("error", err.Error()),
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
