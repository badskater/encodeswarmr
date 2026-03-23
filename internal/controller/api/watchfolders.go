package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// watchFolderStatus augments the config with runtime state reported by the
// API.  The controller has no persistent scan state — last_scan and
// file_count are informational placeholders for future extension.
type watchFolderStatus struct {
	Name              string        `json:"name"`
	Path              string        `json:"path"`
	WindowsPath       string        `json:"windows_path"`
	FilePatterns      []string      `json:"file_patterns"`
	PollInterval      string        `json:"poll_interval"`
	AutoAnalyze       bool          `json:"auto_analyze"`
	MoveAfterAnalysis string        `json:"move_after_analysis,omitempty"`
	Enabled           bool          `json:"enabled"`
	LastScan          *time.Time    `json:"last_scan,omitempty"`
}

// handleListWatchFolders returns all watch folders from configuration.
//
// GET /api/v1/watch-folders
func (s *Server) handleListWatchFolders(w http.ResponseWriter, r *http.Request) {
	folders := s.cfg.WatchFolders
	out := make([]watchFolderStatus, 0, len(folders))
	for _, f := range folders {
		patterns := f.FilePatterns
		if len(patterns) == 0 {
			patterns = []string{"*.mkv", "*.mp4", "*.ts", "*.avi"}
		}
		out = append(out, watchFolderStatus{
			Name:              f.Name,
			Path:              f.Path,
			WindowsPath:       f.WindowsPath,
			FilePatterns:      patterns,
			PollInterval:      f.PollInterval.String(),
			AutoAnalyze:       f.AutoAnalyze,
			MoveAfterAnalysis: f.MoveAfterAnalysis,
			Enabled:           f.Enabled,
		})
	}
	writeJSON(w, r, http.StatusOK, out)
}

// handleToggleWatchFolder enables or disables a watch folder by name.
// The change is applied to the in-memory config only; it does not persist to
// the YAML file.  A server restart will revert to the configured value.
//
// PUT /api/v1/watch-folders/{name}/enable
// PUT /api/v1/watch-folders/{name}/disable
func (s *Server) handleToggleWatchFolder(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	action := r.PathValue("action")
	if name == "" || (action != "enable" && action != "disable") {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid path parameters")
		return
	}

	enabled := action == "enable"

	found := false
	for i := range s.cfg.WatchFolders {
		if s.cfg.WatchFolders[i].Name == name {
			s.cfg.WatchFolders[i].Enabled = enabled
			found = true
			break
		}
	}
	if !found {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "watch folder not found")
		return
	}

	s.logger.Info("watch folder toggled", "name", name, "enabled", enabled)
	writeJSON(w, r, http.StatusOK, map[string]any{"name": name, "enabled": enabled})
}

// handleScanWatchFolder triggers an immediate scan of the named watch folder.
// This is useful for testing or forcing a re-scan without waiting for the
// next poll interval.
//
// POST /api/v1/watch-folders/{name}/scan
func (s *Server) handleScanWatchFolder(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing folder name")
		return
	}

	var found *config.WatchFolderConfig
	for i := range s.cfg.WatchFolders {
		if s.cfg.WatchFolders[i].Name == name {
			found = &s.cfg.WatchFolders[i]
			break
		}
	}
	if found == nil {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "watch folder not found")
		return
	}
	if !found.Enabled {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "watch folder is disabled")
		return
	}
	if s.watcher == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "Service Unavailable",
			"watch folder service is not running")
		return
	}

	go func() {
		ctx := context.Background()
		s.watcher.ScanOne(ctx, *found)
	}()

	writeJSON(w, r, http.StatusAccepted, map[string]any{"name": name, "status": "scan queued"})
}

// watchFolderFormats encodes the list as a top-level JSON array (not wrapped).
// writeJSON already handles this; this helper is used by tests.
func encodeWatchFolders(folders []watchFolderStatus) ([]byte, error) {
	return json.Marshal(folders)
}
