// Package scheduler polls the database for due schedules and creates jobs.
//
// The Scheduler runs a background loop every 30 seconds. When it finds a
// schedule whose next_run_at is at or before now, it decodes the stored
// job_template into a CreateJobParams, submits the job, and advances the
// schedule's last_run_at / next_run_at timestamps using the cron expression.
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/robfig/cron/v3"
)

const pollInterval = 30 * time.Second

// Scheduler polls due schedules and fires jobs on behalf of each one.
type Scheduler struct {
	store  db.Store
	logger *slog.Logger
}

// New creates a new Scheduler. Call Run to start the background loop.
func New(store db.Store, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		store:  store,
		logger: logger.With("component", "scheduler"),
	}
}

// Run starts the polling loop and blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	s.logger.Info("scheduler started", "poll_interval", pollInterval)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Run an initial pass immediately on startup.
	s.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick fetches due schedules and processes each one.
func (s *Scheduler) tick(ctx context.Context) {
	due, err := s.store.ListDueSchedules(ctx)
	if err != nil {
		s.logger.Error("list due schedules", "err", err)
		return
	}
	for _, sc := range due {
		if err := s.fire(ctx, sc); err != nil {
			s.logger.Error("fire schedule", "schedule_id", sc.ID, "name", sc.Name, "err", err)
		}
	}
}

// fire creates a job from the schedule's job_template and advances the
// schedule's run timestamps.
func (s *Scheduler) fire(ctx context.Context, sc *db.Schedule) error {
	// Decode the stored job template into CreateJobParams.
	var params db.CreateJobParams
	if err := json.Unmarshal(sc.JobTemplate, &params); err != nil {
		return fmt.Errorf("scheduler: decode job_template for schedule %s: %w", sc.ID, err)
	}

	// Create the job.
	job, err := s.store.CreateJob(ctx, params)
	if err != nil {
		return fmt.Errorf("scheduler: create job for schedule %s: %w", sc.ID, err)
	}

	s.logger.Info("schedule fired",
		"schedule_id", sc.ID,
		"schedule_name", sc.Name,
		"job_id", job.ID,
	)

	// Compute the next run time from the cron expression.
	nextRunAt, err := nextRun(sc.CronExpr)
	if err != nil {
		// Log but do not return — the job was already created; advance timestamps
		// to now so the scheduler does not re-fire immediately.
		s.logger.Warn("compute next run time", "schedule_id", sc.ID, "err", err)
	}

	now := time.Now().UTC()
	markParams := db.MarkScheduleRunParams{
		ID:        sc.ID,
		LastRunAt: now,
		NextRunAt: nextRunAt,
	}
	if err := s.store.MarkScheduleRun(ctx, markParams); err != nil {
		// Non-fatal: the job was created; the schedule will re-fire on the next
		// poll if MarkScheduleRun fails, but that is preferable to losing the job.
		return fmt.Errorf("scheduler: mark schedule run %s: %w", sc.ID, err)
	}

	return nil
}

// nextRun parses expr and returns the next time after now that it fires.
// Returns nil if the expression cannot be parsed.
func nextRun(expr string) (*time.Time, error) {
	p := cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
	schedule, err := p.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("parse cron expression %q: %w", expr, err)
	}
	t := schedule.Next(time.Now().UTC())
	return &t, nil
}

// NextRunFromExpr is exported for use by the API layer when a schedule is
// created or updated so next_run_at can be pre-populated in the database.
func NextRunFromExpr(expr string) (*time.Time, error) {
	return nextRun(expr)
}
