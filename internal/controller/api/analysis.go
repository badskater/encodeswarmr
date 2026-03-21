package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/db"
)

func (s *Server) handleScanAnalysis(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceID     string `json:"source_id"`
		AnalysisType string `json:"analysis_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.SourceID == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "source_id is required")
		return
	}
	if req.AnalysisType == "" {
		req.AnalysisType = "vmaf"
	}

	job, err := s.store.CreateJob(r.Context(), db.CreateJobParams{
		SourceID: req.SourceID,
		JobType:  "analysis",
		Priority: 0,
	})
	if err != nil {
		s.logger.Error("create analysis job", "err", err, "source_id", req.SourceID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusAccepted, map[string]any{
		"job_id":    job.ID,
		"source_id": req.SourceID,
		"status":    "queued",
	})
}

func (s *Server) handleGetAnalysisResult(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("source_id")
	analysisType := r.URL.Query().Get("type")
	if analysisType == "" {
		analysisType = "vmaf"
	}

	result, err := s.store.GetAnalysisResult(r.Context(), sourceID, analysisType)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "analysis result not found")
		return
	}
	if err != nil {
		s.logger.Error("get analysis result", "err", err, "source_id", sourceID, "type", analysisType)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Decode JSONB fields so they embed cleanly in the response (not base64).
	var frameData json.RawMessage
	if result.FrameData != nil {
		frameData = json.RawMessage(result.FrameData)
	}
	var summary json.RawMessage
	if result.Summary != nil {
		summary = json.RawMessage(result.Summary)
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"id":         result.ID,
		"source_id":  result.SourceID,
		"type":       result.Type,
		"frame_data": frameData,
		"summary":    summary,
		"created_at": result.CreatedAt,
	})
}

func (s *Server) handleListAnalysisResults(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("source_id")

	results, err := s.store.ListAnalysisResults(r.Context(), sourceID)
	if err != nil {
		s.logger.Error("list analysis results", "err", err, "source_id", sourceID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, results)
}
