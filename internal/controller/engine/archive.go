package engine

import (
	"context"
	"log/slog"
	"time"
)

// ArchiveConfig carries archival settings. Kept separate from engine.Config
// for clarity and to avoid polluting the main config struct.
type ArchiveConfig struct {
	// Enabled enables or disables the archival background loop.
	Enabled bool
	// RetentionDays is how many days to keep completed/failed jobs in the
	// active jobs table before archiving them. Defaults to 30 days.
	RetentionDays int
}

// StartArchivalLoop starts a background goroutine that periodically moves
// old completed/failed jobs to the job_archive table. It runs immediately on
// start, then on a 24-hour tick. The goroutine exits when ctx is cancelled.
func (e *Engine) StartArchivalLoop(ctx context.Context, cfg ArchiveConfig) {
	if !cfg.Enabled {
		return
	}
	retention := time.Duration(cfg.RetentionDays) * 24 * time.Hour
	if retention <= 0 {
		retention = 30 * 24 * time.Hour
	}
	go e.archivalLoop(ctx, retention)
}

func (e *Engine) archivalLoop(ctx context.Context, retention time.Duration) {
	// Run immediately on start, then every 24 hours.
	if err := e.archiveCompletedJobs(ctx, retention); err != nil {
		e.logger.Warn("engine: archive completed jobs", "error", err)
	}

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.archiveCompletedJobs(ctx, retention); err != nil {
				e.logger.Warn("engine: archive completed jobs", "error", err)
			}
		}
	}
}

// archiveCompletedJobs moves completed/failed jobs older than the retention
// period to the job_archive table.
func (e *Engine) archiveCompletedJobs(ctx context.Context, retention time.Duration) error {
	n, err := e.store.ArchiveOldJobs(ctx, retention)
	if err != nil {
		return err
	}
	if n > 0 {
		e.logger.LogAttrs(ctx, slog.LevelInfo, "jobs archived",
			slog.Int64("count", n),
			slog.String("retention", retention.String()),
		)
	}
	return nil
}
