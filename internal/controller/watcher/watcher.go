// Package watcher polls configured watch folders for new media files and
// automatically creates source records + schedules analysis jobs.
//
// Watch folders are for ANALYSIS ONLY. No encoding jobs are ever created
// automatically. After analysis completes the source appears in the UI and
// the user manually creates an encoding job with a custom script.
package watcher

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/db"
)

// defaultFilePatterns is used when a watch folder config has no patterns set.
var defaultFilePatterns = []string{"*.mkv", "*.mp4", "*.ts", "*.avi"}

// defaultPollInterval is the fallback when PollInterval is zero.
const defaultPollInterval = 30 * time.Second

// Watcher polls each configured watch folder and creates sources + analysis
// jobs for newly detected files. It never creates encode jobs.
type Watcher struct {
	folders []config.WatchFolderConfig
	store   db.Store
	logger  *slog.Logger

	// seen tracks Linux paths we have already processed in this session to
	// avoid duplicate DB queries on every poll tick.  The canonical check is
	// always GetSourceByUNCPath so this is an optimisation only.
	seen sync.Map
}

// New creates a Watcher from the watch_folders config section.
// Call Start(ctx) to begin polling.
func New(folders []config.WatchFolderConfig, store db.Store, logger *slog.Logger) *Watcher {
	return &Watcher{
		folders: folders,
		store:   store,
		logger:  logger,
	}
}

// Start launches a polling goroutine for each enabled watch folder.
// The goroutines run until ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) {
	for _, f := range w.folders {
		if !f.Enabled {
			w.logger.Info("watcher: folder disabled, skipping", "name", f.Name)
			continue
		}
		interval := f.PollInterval
		if interval <= 0 {
			interval = defaultPollInterval
		}
		w.logger.Info("watcher: starting folder poll",
			"name", f.Name, "path", f.Path, "interval", interval)
		go w.pollLoop(ctx, f, interval)
	}
}

// pollLoop runs the scan/tick loop for a single watch folder.
func (w *Watcher) pollLoop(ctx context.Context, f config.WatchFolderConfig, interval time.Duration) {
	// Scan immediately on startup, then on every tick.
	w.scan(ctx, f)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.scan(ctx, f)
		}
	}
}

// scan lists files in the folder and processes any that have not been seen yet.
func (w *Watcher) scan(ctx context.Context, f config.WatchFolderConfig) {
	patterns := f.FilePatterns
	if len(patterns) == 0 {
		patterns = defaultFilePatterns
	}

	entries, err := os.ReadDir(f.Path)
	if err != nil {
		w.logger.Warn("watcher: read dir failed", "name", f.Name, "path", f.Path, "err", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !matchesAnyPattern(entry.Name(), patterns) {
			continue
		}

		linuxPath := filepath.Join(f.Path, entry.Name())

		// Fast path: already processed in this session.
		if _, seen := w.seen.Load(linuxPath); seen {
			continue
		}

		w.processFile(ctx, f, linuxPath, entry.Name())
	}
}

// processFile checks whether a source already exists for the file; if not,
// creates one and schedules analysis + HDR detect jobs.
func (w *Watcher) processFile(ctx context.Context, f config.WatchFolderConfig, linuxPath, filename string) {
	// Derive the UNC path for the source record so agents can access it.
	uncPath := toUNCPath(linuxPath, f.Path, f.WindowsPath)

	// Check the DB — another controller instance may have created it already.
	existing, err := w.store.GetSourceByUNCPath(ctx, uncPath)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		w.logger.Warn("watcher: check existing source", "path", uncPath, "err", err)
		return
	}
	if existing != nil {
		// Already known; mark as seen so we skip it on future polls.
		w.seen.Store(linuxPath, struct{}{})
		return
	}

	// Create the source record.
	source, err := w.store.CreateSource(ctx, db.CreateSourceParams{
		Filename: filename,
		UNCPath:  uncPath,
	})
	if err != nil {
		w.logger.Warn("watcher: create source failed", "path", uncPath, "err", err)
		return
	}

	// Tag with watch folder name and category.
	category := f.MoveAfterAnalysis
	if category == "" {
		category = "default"
	}
	if err := w.store.UpdateSourceWatch(ctx, db.UpdateSourceWatchParams{
		ID:          source.ID,
		WatchFolder: f.Name,
		Category:    category,
	}); err != nil {
		w.logger.Warn("watcher: update source watch fields", "source_id", source.ID, "err", err)
	}

	w.logger.Info("watcher: new file detected, source created",
		"folder", f.Name, "file", filename, "source_id", source.ID)

	// Optionally schedule analysis (auto_analyze: true is the default intent).
	if f.AutoAnalyze {
		w.scheduleAnalysis(ctx, source.ID)
	}

	w.seen.Store(linuxPath, struct{}{})
}

// scheduleAnalysis creates analysis and hdr_detect jobs for the source.
// Failures are logged as warnings; they can be re-triggered via the UI.
func (w *Watcher) scheduleAnalysis(ctx context.Context, sourceID string) {
	for _, jobType := range []string{"analysis", "hdr_detect"} {
		if _, err := w.store.CreateJob(ctx, db.CreateJobParams{
			SourceID:   sourceID,
			JobType:    jobType,
			TargetTags: []string{},
		}); err != nil {
			w.logger.Warn("watcher: auto-create analysis job failed",
				"source_id", sourceID, "job_type", jobType, "err", err)
		}
	}
	w.logger.Info("watcher: analysis jobs scheduled", "source_id", sourceID)
}

// ScanOne performs a single immediate scan of the given folder.
// It is exported so the API can trigger an on-demand scan.
func (w *Watcher) ScanOne(ctx context.Context, f config.WatchFolderConfig) {
	w.scan(ctx, f)
}

// matchesAnyPattern reports whether name matches at least one glob pattern.
func matchesAnyPattern(name string, patterns []string) bool {
	for _, pat := range patterns {
		matched, err := filepath.Match(pat, name)
		if err == nil && matched {
			return true
		}
		// Also try case-insensitive match for Windows-style patterns.
		matched, err = filepath.Match(strings.ToLower(pat), strings.ToLower(name))
		if err == nil && matched {
			return true
		}
	}
	return false
}

// toUNCPath converts a Linux file path to a Windows UNC path using the
// configured base paths.  If no substitution is possible the Linux path is
// returned unchanged (callers can still store it; agents may not be able to
// reach it without a path mapping).
func toUNCPath(linuxPath, linuxBase, windowsBase string) string {
	if windowsBase == "" || linuxBase == "" {
		return linuxPath
	}
	rel, err := filepath.Rel(linuxBase, linuxPath)
	if err != nil {
		return linuxPath
	}
	// Convert forward slashes to backslashes for the UNC portion.
	rel = strings.ReplaceAll(rel, "/", `\`)
	return windowsBase + `\` + rel
}
