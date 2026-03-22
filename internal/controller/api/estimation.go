package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/controller/presets"
	"github.com/badskater/encodeswarmr/internal/db"
)

// estimateRequest is the payload accepted by POST /api/v1/estimate.
type estimateRequest struct {
	SourceID   string `json:"source_id"`
	Codec      string `json:"codec"`
	PresetName string `json:"preset_name"`
	ChunkCount int    `json:"chunk_count"`
}

// estimateResponse is the payload returned by POST /api/v1/estimate.
type estimateResponse struct {
	EstimatedDurationSeconds int64  `json:"estimated_duration_seconds"`
	EstimatedDurationHuman   string `json:"estimated_duration_human"`
	// Confidence is "high" (30+ samples), "medium" (5–29), "low" (1–4), or
	// "none" when no historical data is available and defaults are used.
	Confidence     string `json:"confidence"`
	BasedOnSamples int64  `json:"based_on_samples"`
	// AvgFPS is the average encoding FPS used for the estimate.
	AvgFPS float64 `json:"avg_fps"`
	// FPSStddev is the standard deviation of avg_fps across historical samples.
	// A 95% CI for the estimate can be computed as ±1.96 * fps_stddev / sqrt(sample_count).
	FPSStddev float64 `json:"fps_stddev,omitempty"`
	// EstimatedStorageMB is the predicted output file size in megabytes.
	EstimatedStorageMB float64 `json:"estimated_storage_mb,omitempty"`
}

// handleEstimate computes a rough time estimate for an encode job.
//
//	POST /api/v1/estimate
func (s *Server) handleEstimate(w http.ResponseWriter, r *http.Request) {
	var req estimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	// Validate source exists.
	if req.SourceID == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "source_id is required")
		return
	}
	if _, srcErr := s.store.GetSourceByID(r.Context(), req.SourceID); srcErr != nil {
		if errors.Is(srcErr, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "source not found")
			return
		}
		s.logger.Error("estimate: get source", "err", srcErr)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Determine total source frames from analysis summary or fall back to 0.
	totalFrames := int64(0)
	analysisResults, err := s.store.ListAnalysisResults(r.Context(), req.SourceID)
	if err != nil {
		s.logger.Error("estimate: list analysis results", "err", err)
		// Non-fatal: proceed with zero frames, estimate will be rough.
	}
	for _, ar := range analysisResults {
		if len(ar.Summary) == 0 {
			continue
		}
		var summary map[string]any
		if err := json.Unmarshal(ar.Summary, &summary); err != nil {
			continue
		}
		if frames, ok := summary["total_frames"]; ok {
			switch v := frames.(type) {
			case float64:
				totalFrames = int64(v)
			case int64:
				totalFrames = v
			}
			break
		}
	}

	// Resolve codec: prefer preset lookup, fall back to explicit codec field.
	codec := req.Codec
	if req.PresetName != "" {
		p := presets.Get(req.PresetName)
		if p == nil {
			writeProblem(w, r, http.StatusBadRequest, "Bad Request", "preset not found")
			return
		}
		codec = p.Codec
	}
	if codec == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "codec or preset_name is required")
		return
	}

	chunkCount := req.ChunkCount
	if chunkCount < 1 {
		chunkCount = 1
	}

	// Query historical FPS data for this source.
	avgFPS, sampleCount, err := s.store.GetAvgFPSStats(r.Context(), req.SourceID)
	if err != nil {
		s.logger.Error("estimate: get avg fps stats", "err", err)
		// Non-fatal: fall back to codec defaults.
		avgFPS = 0
		sampleCount = 0
	}

	confidence := "none"
	if sampleCount == 0 || avgFPS == 0 {
		// No historical data — use codec default.
		if def, ok := presets.DefaultFPSByCodec[codec]; ok {
			avgFPS = def
		} else {
			avgFPS = 10.0 // conservative fallback for unknown codecs
		}
		confidence = "none"
	} else {
		switch {
		case sampleCount >= 30:
			confidence = "high"
		case sampleCount >= 5:
			confidence = "medium"
		default:
			confidence = "low"
		}
	}

	// estimated_seconds = total_frames / avg_fps / chunk_count
	// If we have no frame count, we cannot estimate; return 0 with confidence=none.
	estimatedSeconds := int64(0)
	if totalFrames > 0 && avgFPS > 0 {
		estimatedSeconds = int64(math.Ceil(float64(totalFrames) / avgFPS / float64(chunkCount)))
	}

	// Augment with EncodingStats (learning) for confidence intervals and
	// storage estimate.  Use codec+empty resolution+empty preset as the
	// broadest match; callers may supply resolution/preset in future.
	var fpsStddev float64
	var estimatedStorageMB float64
	if es, err := s.store.GetEncodingStats(r.Context(), codec, "", ""); err == nil && es != nil {
		fpsStddev = es.FPSStddev
		// Use the per-stat sample count if it's larger (cross-source stats).
		if int64(es.SampleCount) > sampleCount {
			sampleCount = int64(es.SampleCount)
			avgFPS = es.AvgFPS
			switch {
			case sampleCount >= 30:
				confidence = "high"
			case sampleCount >= 5:
				confidence = "medium"
			default:
				confidence = "low"
			}
		}
		// Estimated storage: avg_size_per_min * estimated_minutes.
		if es.AvgSizePerMin > 0 && estimatedSeconds > 0 {
			estimatedStorageMB = es.AvgSizePerMin * float64(estimatedSeconds) / 60.0 / (1024 * 1024)
		}
	}

	resp := estimateResponse{
		EstimatedDurationSeconds: estimatedSeconds,
		EstimatedDurationHuman:   formatDuration(estimatedSeconds),
		Confidence:               confidence,
		BasedOnSamples:           sampleCount,
		AvgFPS:                   avgFPS,
		FPSStddev:                fpsStddev,
		EstimatedStorageMB:       estimatedStorageMB,
	}
	writeJSON(w, r, http.StatusOK, resp)
}

// formatDuration converts a duration in seconds to a human-readable string
// such as "1h 30m" or "45m 0s".
func formatDuration(secs int64) string {
	if secs <= 0 {
		return "unknown"
	}
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm %ds", m, s)
}
