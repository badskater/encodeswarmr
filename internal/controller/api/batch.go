package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/badskater/encodeswarmr/internal/controller/analysis"
	"github.com/badskater/encodeswarmr/internal/db"
	"github.com/badskater/encodeswarmr/internal/shared"
)

// batchImportRequest is the body for POST /api/v1/sources/batch-import.
type batchImportRequest struct {
	// PathPattern is a glob pattern. UNC paths are translated via path_mappings
	// before globbing so the controller can access the NAS via its NFS mounts.
	PathPattern    string `json:"path_pattern"`
	Recursive      bool   `json:"recursive"`
	AutoAnalyze    bool   `json:"auto_analyze"`
	AutoEncode     bool   `json:"auto_encode"`
	// EncodeTemplateID is the run-script template to use when auto_encode=true.
	EncodeTemplateID string `json:"encode_template_id,omitempty"`
}

// batchImportResult summarises one file processed by the batch operation.
type batchImportResult struct {
	Path       string `json:"path"`
	SourceID   string `json:"source_id,omitempty"`
	JobID      string `json:"job_id,omitempty"`
	Skipped    bool   `json:"skipped,omitempty"`
	SkipReason string `json:"skip_reason,omitempty"`
	Error      string `json:"error,omitempty"`
}

// handleBatchImport scans a path pattern, creates a Source for each match,
// and optionally queues analysis or encode jobs.
//
// POST /api/v1/sources/batch-import
func (s *Server) handleBatchImport(w http.ResponseWriter, r *http.Request) {
	var req batchImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	if req.PathPattern == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "path_pattern is required")
		return
	}

	ctx := r.Context()

	// Translate the UNC base directory to a local Linux mount path.
	resolvedPattern := s.translatePatternPath(ctx, req.PathPattern)

	// Glob the pattern to find matching files.
	matches, err := s.globPattern(resolvedPattern, req.Recursive)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request",
			"invalid path_pattern: "+err.Error())
		return
	}

	// Fetch path mappings once for reverse translation.
	mappings, _ := s.store.ListPathMappings(ctx)

	results := make([]batchImportResult, 0, len(matches))
	for _, localPath := range matches {
		// Reverse-map the local path back to the UNC/NAS path for storage.
		uncPath := reverseMapPath(localPath, mappings)
		if !shared.IsSharePath(uncPath) {
			uncPath = localPath // keep original if no reverse map
		}

		filename := filepath.Base(uncPath)

		// Idempotent: skip if source already registered.
		existing, checkErr := s.store.GetSourceByUNCPath(ctx, uncPath)
		if checkErr == nil && existing != nil {
			results = append(results, batchImportResult{
				Path:       localPath,
				SourceID:   existing.ID,
				Skipped:    true,
				SkipReason: "source already exists",
			})
			continue
		}

		source, createErr := s.store.CreateSource(ctx, db.CreateSourceParams{
			Filename: filename,
			UNCPath:  uncPath,
		})
		if createErr != nil {
			s.logger.Error("batch import: create source", "path", localPath, "err", createErr)
			results = append(results, batchImportResult{
				Path:  localPath,
				Error: createErr.Error(),
			})
			continue
		}

		res := batchImportResult{
			Path:     localPath,
			SourceID: source.ID,
		}

		if req.AutoAnalyze {
			s.scheduleSourceAnalysis(ctx, source.ID)
		}

		if req.AutoEncode && req.EncodeTemplateID != "" {
			job, encErr := s.store.CreateJob(ctx, db.CreateJobParams{
				SourceID: source.ID,
				JobType:  "encode",
				EncodeConfig: db.EncodeConfig{
					RunScriptTemplateID: req.EncodeTemplateID,
				},
				TargetTags: []string{},
			})
			if encErr != nil {
				s.logger.Error("batch import: create encode job",
					"source_id", source.ID, "err", encErr)
			} else {
				res.JobID = job.ID
			}
		}

		results = append(results, res)
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"imported": len(matches),
		"results":  results,
	})
}

// translatePatternPath resolves UNC path prefixes in a glob pattern to local
// Linux mount paths using the path_mappings table. The glob part (e.g. *.mkv)
// is preserved.
func (s *Server) translatePatternPath(ctx context.Context, pattern string) string {
	mappings, err := s.store.ListPathMappings(ctx)
	if err != nil || len(mappings) == 0 {
		return pattern
	}
	return analysis.ApplyMappings(pattern, mappings)
}

// reverseMapPath converts a Linux local path back to its Windows UNC equivalent
// using the path_mappings table.  Returns the original path if no mapping matches.
func reverseMapPath(localPath string, mappings []*db.PathMapping) string {
	for _, m := range mappings {
		if !m.Enabled {
			continue
		}
		linuxBase := strings.TrimRight(m.LinuxPrefix, "/")
		if !strings.HasPrefix(localPath, linuxBase) {
			continue
		}
		remainder := localPath[len(linuxBase):]
		remainder = strings.ReplaceAll(remainder, "/", `\`)
		winBase := strings.TrimRight(m.WindowsPrefix, `\`)
		return winBase + `\` + strings.TrimLeft(remainder, `\`)
	}
	return localPath
}

// globPattern returns a list of files matching the given pattern.
// When recursive is true the directory is walked and the pattern extension is
// used as a filter.
func (s *Server) globPattern(pattern string, recursive bool) ([]string, error) {
	if !recursive {
		return filepath.Glob(pattern)
	}

	dir := filepath.Dir(pattern)
	ext := filepath.Ext(filepath.Base(pattern))

	var matches []string
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() && (ext == "" || strings.EqualFold(filepath.Ext(path), ext)) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, walkErr
}
