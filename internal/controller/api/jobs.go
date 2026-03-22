package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/badskater/encodeswarmr/internal/controller/webhooks"
	"github.com/badskater/encodeswarmr/internal/db"
)

// handleListJobs returns a paginated list of jobs, optionally filtered by status.
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	// Parse page_size with default 50, max 200.
	pageSize := 50
	if ps := r.URL.Query().Get("page_size"); ps != "" {
		n, err := strconv.Atoi(ps)
		if err != nil || n < 1 {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "page_size must be a positive integer")
			return
		}
		pageSize = n
		if pageSize > 200 {
			pageSize = 200
		}
	}

	// Decode base64 cursor if present.
	var cursor string
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid cursor")
			return
		}
		cursor = string(decoded)
	}

	jobs, total, err := s.store.ListJobs(r.Context(), db.ListJobsFilter{
		Status:   status,
		Cursor:   cursor,
		PageSize: pageSize,
	})
	if err != nil {
		s.logger.Error("list jobs", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	var nextCursor string
	if len(jobs) == pageSize {
		nextCursor = base64.StdEncoding.EncodeToString([]byte(jobs[len(jobs)-1].ID))
	}
	writeCollection(w, r, jobs, total, nextCursor)
}

// validJobTypes is the set of job types accepted by handleCreateJob.
var validJobTypes = map[string]bool{
	"encode":     true,
	"analysis":   true,
	"audio":      true,
	"hdr_detect": true,
	"merge":      true,
}

// handleCreateJob creates a new encoding or analysis job.
func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceID     string           `json:"source_id"`
		JobType      string           `json:"job_type"`
		Priority     int              `json:"priority"`
		TargetTags   []string         `json:"target_tags"`
		EncodeConfig db.EncodeConfig  `json:"encode_config"`
		AudioConfig  *db.AudioConfig  `json:"audio_config,omitempty"`
		// FlowID is an optional flow pipeline to use for job expansion.
		FlowID    string  `json:"flow_id,omitempty"`
		DependsOn *string `json:"depends_on,omitempty"`
		ChainGroup *string `json:"chain_group,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	if req.SourceID == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "source_id is required")
		return
	}
	if !validJobTypes[req.JobType] {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			`job_type must be "encode", "analysis", "audio", "hdr_detect", or "merge"`)
		return
	}

	// Validate audio_config codec when job_type is audio.
	if req.JobType == "audio" && req.AudioConfig != nil {
		if err := validateAudioConfig(req.AudioConfig); err != nil {
			writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", err.Error())
			return
		}
	}

	// When a flow_id is provided, skip the template-based validation — the
	// flow engine will determine task structure at expansion time.
	if req.FlowID == "" && req.JobType == "encode" {
		if req.EncodeConfig.RunScriptTemplateID == "" {
			writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "encode_config.run_script_template_id is required for encode jobs")
			return
		}
		if len(req.EncodeConfig.ChunkBoundaries) == 0 {
			writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "encode_config.chunk_boundaries must not be empty for encode jobs")
			return
		}
	}

	// Propagate flow_id into EncodeConfig so it travels with the job record.
	if req.FlowID != "" {
		req.EncodeConfig.FlowID = req.FlowID
	}

	job, err := s.store.CreateJob(r.Context(), db.CreateJobParams{
		SourceID:     req.SourceID,
		JobType:      req.JobType,
		Priority:     req.Priority,
		TargetTags:   req.TargetTags,
		EncodeConfig: req.EncodeConfig,
		AudioConfig:  req.AudioConfig,
		DependsOn:    req.DependsOn,
		ChainGroup:   req.ChainGroup,
	})
	if err != nil {
		s.logger.Error("create job", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusCreated, job)
}

// validAudioCodecs is the set of ffmpeg codec names accepted for audio jobs.
var validAudioCodecs = map[string]bool{
	"flac":        true,
	"libopus":     true,
	"libfdk_aac":  true,
	"aac":         true, // aac-lc (native ffmpeg encoder)
	"ac3":         true,
	"eac3":        true,
	"dca":         true, // DTS
	"truehd":      true,
	"pcm_s16le":   true,
	"pcm_s24le":   true,
	"libmp3lame":  true,
	"libvorbis":   true,
}

func validateAudioConfig(cfg *db.AudioConfig) error {
	if cfg.Codec == "" {
		return fmt.Errorf("audio_config.codec is required")
	}
	if !validAudioCodecs[cfg.Codec] {
		return fmt.Errorf("audio_config.codec %q is not supported; valid: flac, libopus, libfdk_aac, aac, ac3, eac3, dca, truehd, pcm_s16le, pcm_s24le, libmp3lame, libvorbis", cfg.Codec)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Job chain creation
// ---------------------------------------------------------------------------

// chainStep describes one step in a job chain request.
type chainStep struct {
	JobType      string           `json:"job_type"`
	Name         string           `json:"name"`
	Priority     int              `json:"priority"`
	TargetTags   []string         `json:"target_tags"`
	EncodeConfig *db.EncodeConfig `json:"encode_config,omitempty"`
	AudioConfig  *db.AudioConfig  `json:"audio_config,omitempty"`
}

// handleCreateJobChain creates multiple jobs with sequential depends_on links
// and a shared chain_group UUID.
//
// POST /api/v1/job-chains
// Body: { "source_id": "uuid", "steps": [ { "job_type": "...", ...}, ... ] }
func (s *Server) handleCreateJobChain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceID string      `json:"source_id"`
		Steps    []chainStep `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	if req.SourceID == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "source_id is required")
		return
	}
	if len(req.Steps) == 0 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "steps must not be empty")
		return
	}

	// Validate each step.
	for i, step := range req.Steps {
		if !validJobTypes[step.JobType] {
			writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
				fmt.Sprintf("steps[%d]: unsupported job_type %q", i, step.JobType))
			return
		}
		if step.JobType == "audio" && step.AudioConfig != nil {
			if err := validateAudioConfig(step.AudioConfig); err != nil {
				writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
					fmt.Sprintf("steps[%d]: %s", i, err.Error()))
				return
			}
		}
	}

	// Generate a shared chain_group UUID using crypto/rand.
	chainGroupID := newUUID()

	ctx := r.Context()
	created := make([]*db.Job, 0, len(req.Steps))
	var prevJobID *string

	for _, step := range req.Steps {
		cfg := db.EncodeConfig{}
		if step.EncodeConfig != nil {
			cfg = *step.EncodeConfig
		}

		tags := step.TargetTags
		if tags == nil {
			tags = []string{}
		}

		job, err := s.store.CreateJob(ctx, db.CreateJobParams{
			SourceID:     req.SourceID,
			JobType:      step.JobType,
			Priority:     step.Priority,
			TargetTags:   tags,
			EncodeConfig: cfg,
			AudioConfig:  step.AudioConfig,
			DependsOn:    prevJobID,
			ChainGroup:   &chainGroupID,
		})
		if err != nil {
			s.logger.Error("create chain job", "err", err, "job_type", step.JobType)
			writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
			return
		}

		created = append(created, job)
		id := job.ID
		prevJobID = &id
	}

	writeJSON(w, r, http.StatusCreated, map[string]any{
		"chain_group": chainGroupID,
		"jobs":        created,
	})
}

// handleGetJobChain returns all jobs in a chain group.
//
// GET /api/v1/job-chains/{chain_group}
func (s *Server) handleGetJobChain(w http.ResponseWriter, r *http.Request) {
	chainGroup := r.PathValue("chain_group")
	if chainGroup == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing chain_group")
		return
	}

	jobs, err := s.store.ListJobsByChainGroup(r.Context(), chainGroup)
	if err != nil {
		s.logger.Error("list jobs by chain group", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"chain_group": chainGroup,
		"jobs":        jobs,
	})
}

// handleGetJob returns a single job with its tasks.
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing job id")
		return
	}

	job, err := s.store.GetJobByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "job not found")
			return
		}
		s.logger.Error("get job", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	tasks, err := s.store.ListTasksByJob(r.Context(), id)
	if err != nil {
		s.logger.Error("list tasks for job", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"job":   job,
		"tasks": tasks,
	})
}

// handleCancelJob cancels a job and its pending tasks.
func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing job id")
		return
	}

	job, err := s.store.GetJobByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "job not found")
			return
		}
		s.logger.Error("get job for cancel", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	if job.Status == "cancelled" || job.Status == "completed" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "job is already "+job.Status)
		return
	}

	if err := s.store.UpdateJobStatus(r.Context(), id, "cancelled"); err != nil {
		s.logger.Error("cancel job", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if err := s.store.CancelPendingTasksForJob(r.Context(), id); err != nil {
		s.logger.Error("cancel pending tasks", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	s.webhooks.Emit(r.Context(), webhooks.Event{
		Type:    "job.cancelled",
		Payload: map[string]any{"job_id": id, "source_id": job.SourceID},
	})

	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

// handleRetryJob retries all failed tasks in a failed job.
func (s *Server) handleRetryJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing job id")
		return
	}

	job, err := s.store.GetJobByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "job not found")
			return
		}
		s.logger.Error("get job for retry", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	if job.Status != "failed" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "only failed jobs can be retried")
		return
	}

	if err := s.store.RetryFailedTasksForJob(r.Context(), id); err != nil {
		s.logger.Error("retry failed tasks", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if err := s.store.UpdateJobStatus(r.Context(), id, "queued"); err != nil {
		s.logger.Error("update job status to queued", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

// handleGetTask returns a single task by ID.
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing task id")
		return
	}

	task, err := s.store.GetTaskByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "task not found")
			return
		}
		s.logger.Error("get task", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, task)
}
