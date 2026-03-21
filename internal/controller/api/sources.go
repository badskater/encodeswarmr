package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/badskater/distributed-encoder/internal/shared"
)

func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path     string  `json:"path"`
		Name     string  `json:"name"`
		CloudURI *string `json:"cloud_uri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	// cloud_uri and path are mutually exclusive. At least one is required.
	hasCloudURI := req.CloudURI != nil && *req.CloudURI != ""
	hasPath := req.Path != ""

	if !hasCloudURI && !hasPath {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "either path or cloud_uri is required")
		return
	}
	if hasCloudURI && hasPath {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "path and cloud_uri are mutually exclusive")
		return
	}

	if hasPath && !shared.IsSharePath(req.Path) {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "path must be a UNC path (\\\\server\\share\\...) or NFS mount path (/mnt/nas/...)")
		return
	}
	if hasCloudURI && !isCloudURI(*req.CloudURI) {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "cloud_uri must use a supported scheme: s3://, gs://, or az://")
		return
	}

	filename := req.Name
	if filename == "" {
		if hasPath {
			filename = filepath.Base(req.Path)
		} else {
			filename = filepath.Base(*req.CloudURI)
		}
	}

	// Duplicate detection: for UNC-path sources we check by path; for cloud
	// sources the cloud_uri acts as the unique key.
	if hasPath {
		existing, err := s.store.GetSourceByUNCPath(r.Context(), req.Path)
		if err != nil && !errors.Is(err, db.ErrNotFound) {
			s.logger.Error("check source by unc path", "err", err)
			writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
			return
		}
		if existing != nil {
			writeJSON(w, r, http.StatusOK, existing)
			return
		}
	}

	source, err := s.store.CreateSource(r.Context(), db.CreateSourceParams{
		Filename: filename,
		UNCPath:  req.Path,
		CloudURI: req.CloudURI,
	})
	if err != nil {
		s.logger.Error("create source", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Automatically queue scene detection / VMAF analysis and HDR/DV detection
	// for every new source.  Encode script generation is deferred until the
	// operator explicitly submits an encode job.
	s.scheduleSourceAnalysis(r.Context(), source.ID)

	writeJSON(w, r, http.StatusCreated, source)
}

// isCloudURI returns true when uri uses a supported cloud storage scheme.
func isCloudURI(uri string) bool {
	lower := strings.ToLower(uri)
	return strings.HasPrefix(lower, "s3://") ||
		strings.HasPrefix(lower, "gs://") ||
		strings.HasPrefix(lower, "az://")
}

// scheduleSourceAnalysis creates an analysis job (VMAF + scene detection) and
// an hdr_detect job for the given source.  Failures are logged as warnings and
// do not affect the caller — the jobs can be re-triggered manually via the
// individual source endpoints if needed.
func (s *Server) scheduleSourceAnalysis(ctx context.Context, sourceID string) {
	for _, jobType := range []string{"analysis", "hdr_detect"} {
		if _, err := s.store.CreateJob(ctx, db.CreateJobParams{
			SourceID: sourceID,
			JobType:  jobType,
		}); err != nil {
			s.logger.Warn("auto-create analysis job failed",
				"source_id", sourceID, "job_type", jobType, "err", err)
		}
	}
}

func (s *Server) handleAnalyzeSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := s.store.GetSourceByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "source not found")
		return
	}
	if err != nil {
		s.logger.Error("get source for analyze", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	job, err := s.store.CreateJob(r.Context(), db.CreateJobParams{
		SourceID: id,
		JobType:  "analysis",
	})
	if err != nil {
		s.logger.Error("create analysis job", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusCreated, job)
}

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	cursorParam := r.URL.Query().Get("cursor")

	pageSize := 50
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil || n < 1 {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "page_size must be a positive integer")
			return
		}
		if n > 200 {
			n = 200
		}
		pageSize = n
	}

	var decoded string
	if cursorParam != "" {
		raw, err := base64.StdEncoding.DecodeString(cursorParam)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid cursor")
			return
		}
		decoded = string(raw)
	}

	sources, totalCount, err := s.store.ListSources(r.Context(), db.ListSourcesFilter{
		State:    state,
		Cursor:   decoded,
		PageSize: pageSize,
	})
	if err != nil {
		s.logger.Error("list sources", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	var nextCursor string
	if len(sources) == pageSize {
		nextCursor = base64.StdEncoding.EncodeToString([]byte(sources[len(sources)-1].ID))
	}

	writeCollection(w, r, sources, totalCount, nextCursor)
}

func (s *Server) handleGetSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	source, err := s.store.GetSourceByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "source not found")
		return
	}
	if err != nil {
		s.logger.Error("get source", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	results, err := s.store.ListAnalysisResults(r.Context(), id)
	if err != nil {
		s.logger.Error("list analysis results for source", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"source":           source,
		"analysis_results": results,
	})
}

func (s *Server) handleEncodeSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Priority   int      `json:"priority"`
		TargetTags []string `json:"target_tags"`
		JobType    string   `json:"job_type"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
			return
		}
	}
	if req.JobType == "" {
		req.JobType = "encode"
	}

	_, err := s.store.GetSourceByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "source not found")
		return
	}
	if err != nil {
		s.logger.Error("get source for encode", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	job, err := s.store.CreateJob(r.Context(), db.CreateJobParams{
		SourceID:   id,
		JobType:    req.JobType,
		Priority:   req.Priority,
		TargetTags: req.TargetTags,
	})
	if err != nil {
		s.logger.Error("create encode job", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusCreated, job)
}

// handleHDRDetectSource creates an hdr_detect job for the source.  The job
// runs ffprobe (and optionally dovi_tool) on the agent, then the controller
// parses the result and updates sources.hdr_type / sources.dv_profile.
//
// POST /api/v1/sources/{id}/hdr-detect
func (s *Server) handleHDRDetectSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := s.store.GetSourceByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "source not found")
		return
	}
	if err != nil {
		s.logger.Error("get source for hdr_detect", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	job, err := s.store.CreateJob(r.Context(), db.CreateJobParams{
		SourceID: id,
		JobType:  "hdr_detect",
		Priority: 0,
	})
	if err != nil {
		s.logger.Error("create hdr_detect job", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusCreated, map[string]any{
		"job_id":    job.ID,
		"source_id": id,
		"status":    "queued",
	})
}

// handleUpdateSourceHDR sets the hdr_type and dv_profile on a source.
// Operators call this after running an hdr_detect analysis job or when
// providing the values manually.
//
// PATCH /api/v1/sources/{id}/hdr
// Body: { "hdr_type": "hdr10", "dv_profile": 0 }
func (s *Server) handleUpdateSourceHDR(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		HDRType   string `json:"hdr_type"`
		DVProfile int    `json:"dv_profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	// Validate hdr_type against the known set.
	validHDRTypes := map[string]bool{
		"":             true,
		"hdr10":        true,
		"hdr10+":       true,
		"dolby_vision": true,
		"hlg":          true,
	}
	if !validHDRTypes[req.HDRType] {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request",
			"hdr_type must be one of: hdr10, hdr10+, dolby_vision, hlg, or empty string for SDR")
		return
	}
	if req.DVProfile < 0 {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "dv_profile must be >= 0")
		return
	}

	err := s.store.UpdateSourceHDR(r.Context(), db.UpdateSourceHDRParams{
		ID:        id,
		HDRType:   req.HDRType,
		DVProfile: req.DVProfile,
	})
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "source not found")
		return
	}
	if err != nil {
		s.logger.Error("update source hdr", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	source, err := s.store.GetSourceByID(r.Context(), id)
	if err != nil {
		s.logger.Error("get source after hdr update", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, source)
}

func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.DeleteSource(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "source not found")
		return
	}
	if err != nil {
		s.logger.Error("delete source", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetSourceScenes returns the scene boundaries derived from the most
// recent scene-analysis result for the source.  Each boundary includes the
// frame number, PTS value, and a formatted HH:MM:SS.ff timecode.
//
// GET /api/v1/sources/{id}/scenes
func (s *Server) handleGetSourceScenes(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := s.store.GetSourceByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "source not found")
		return
	}
	if err != nil {
		s.logger.Error("get source for scenes", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	result, err := s.store.GetAnalysisResult(r.Context(), id, "scene")
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "no scene analysis available for this source — run an analysis job first")
		return
	}
	if err != nil {
		s.logger.Error("get scene analysis result", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Decode the stored JSONB frame_data array.
	type framePoint struct {
		Frame *int     `json:"frame"`
		PTS   *float64 `json:"pts"`
	}
	var frameData []framePoint
	if len(result.FrameData) > 0 {
		if err := json.Unmarshal(result.FrameData, &frameData); err != nil {
			s.logger.Error("unmarshal scene frame_data", "err", err, "source_id", id)
			writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
			return
		}
	}

	// Decode the stored JSONB summary to derive FPS and total frame count.
	type summary struct {
		FrameCount  *int     `json:"frame_count"`
		DurationSec *float64 `json:"duration_sec"`
	}
	var sum summary
	if len(result.Summary) > 0 {
		_ = json.Unmarshal(result.Summary, &sum) // best-effort
	}

	fps := 24.0
	totalFrames := 0
	durationSec := 0.0
	if sum.FrameCount != nil && sum.DurationSec != nil && *sum.DurationSec > 0 {
		fps = float64(*sum.FrameCount) / *sum.DurationSec
		totalFrames = *sum.FrameCount
		durationSec = *sum.DurationSec
	}

	type sceneBoundary struct {
		Frame    int     `json:"frame"`
		PTS      float64 `json:"pts"`
		Timecode string  `json:"timecode"`
	}

	scenes := make([]sceneBoundary, 0, len(frameData))
	for _, fp := range frameData {
		frame := 0
		pts := 0.0
		if fp.Frame != nil {
			frame = *fp.Frame
			pts = float64(frame) / fps
		} else if fp.PTS != nil {
			pts = *fp.PTS
			frame = int(pts * fps)
		} else {
			continue
		}
		scenes = append(scenes, sceneBoundary{
			Frame:    frame,
			PTS:      pts,
			Timecode: ptsToTimecode(pts, fps),
		})
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"source_id":    id,
		"fps":          fps,
		"total_frames": totalFrames,
		"duration_sec": durationSec,
		"scenes":       scenes,
	})
}

// ptsToTimecode formats a PTS (seconds) value as HH:MM:SS.ff using the given
// frame rate to compute the sub-second frame component.
func ptsToTimecode(pts, fps float64) string {
	h := int(pts) / 3600
	m := (int(pts) % 3600) / 60
	sec := int(pts) % 60
	frame := int((pts-float64(int(pts)))*fps + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d.%02d", h, m, sec, frame)
}
