package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// Config holds the settings for the background engine loop.
type Config struct {
	DispatchInterval   time.Duration
	StaleThreshold     time.Duration
	ScriptBaseDir      string
	// LogRetention is how long to keep task log rows. 0 disables log pruning.
	LogRetention       time.Duration
	// LogCleanupInterval controls how often the log retention loop runs.
	// Defaults to 1 hour when 0.
	LogCleanupInterval time.Duration
}

// AnalysisRunner is the interface used by the engine to execute analysis,
// HDR-detect, and audio jobs on the controller host.
type AnalysisRunner interface {
	RunHDRDetect(ctx context.Context, job *db.Job, source *db.Source) error
	RunAnalysis(ctx context.Context, job *db.Job, source *db.Source) error
	RunAudio(ctx context.Context, job *db.Job, source *db.Source) error
}

// ConcatRunner executes the final ffmpeg concat on the controller after
// all chunk encode tasks complete.
type ConcatRunner interface {
	RunConcat(ctx context.Context, job *db.Job, chunkPaths []string, outputPath string) error
}

// Engine orchestrates job expansion and stale-agent detection on a timer.
type Engine struct {
	store    db.Store
	gen      *ScriptGenerator
	cfg      Config
	logger   *slog.Logger
	analysis AnalysisRunner // optional; nil falls back to agent dispatch
	concat   ConcatRunner   // optional; nil falls back to agent dispatch
}

// New creates an Engine. Does not start the background loop.
func New(store db.Store, cfg Config, logger *slog.Logger) *Engine {
	return &Engine{
		store:  store,
		gen:    newScriptGenerator(store, cfg.ScriptBaseDir, logger),
		cfg:    cfg,
		logger: logger,
	}
}

// SetAnalysisRunner attaches a controller-side analysis runner.  When set,
// analysis/hdr_detect/audio jobs run on the controller instead of being
// dispatched to an agent.
func (e *Engine) SetAnalysisRunner(r AnalysisRunner) {
	e.analysis = r
}

// SetConcatRunner attaches a controller-side concat runner.  When set,
// the final ffmpeg concat step runs on the controller instead of being
// dispatched to an agent.
func (e *Engine) SetConcatRunner(r ConcatRunner) {
	e.concat = r
}

// Start launches the background dispatch loop in a goroutine.
// Returns immediately. The loop runs until ctx is cancelled.
func (e *Engine) Start(ctx context.Context) {
	go e.loop(ctx)
	if e.cfg.LogRetention > 0 {
		go e.logRetentionLoop(ctx)
	}
}

func (e *Engine) loop(ctx context.Context) {
	ticker := time.NewTicker(e.cfg.DispatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.expandPendingJobs(ctx); err != nil {
				e.logger.Warn("engine: expand pending jobs", "error", err)
			}
			if err := e.checkStaleAgents(ctx); err != nil {
				e.logger.Warn("engine: check stale agents", "error", err)
			}
		}
	}
}

// logRetentionLoop periodically prunes task log rows older than LogRetention.
func (e *Engine) logRetentionLoop(ctx context.Context) {
	interval := e.cfg.LogCleanupInterval
	if interval <= 0 {
		interval = time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-e.cfg.LogRetention)
			if err := e.store.PruneOldTaskLogs(ctx, cutoff); err != nil {
				e.logger.Warn("engine: prune old task logs", "error", err)
			} else {
				e.logger.Debug("engine: pruned task logs older than retention period",
					"cutoff", cutoff,
					"retention", e.cfg.LogRetention,
				)
			}
		}
	}
}
