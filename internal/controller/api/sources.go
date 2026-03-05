package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/badskater/distributed-encoder/internal/shared"
)

func (s *Server) handleCreateSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if !shared.IsUNCPath(req.Path) {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "path must be a UNC path (\\\\server\\share\\...)")
		return
	}
	filename := req.Name
	if filename == "" {
		filename = filepath.Base(req.Path)
	}

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

	source, err := s.store.CreateSource(r.Context(), db.CreateSourceParams{
		Filename: filename,
		UNCPath:  req.Path,
	})
	if err != nil {
		s.logger.Error("create source", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusCreated, source)
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
