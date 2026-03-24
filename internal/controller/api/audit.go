package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// handleListAuditLog returns paginated audit log entries.
// GET /api/v1/audit-log
func (s *Server) handleListAuditLog(w http.ResponseWriter, r *http.Request) {
	limit := 100
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	entries, total, err := s.store.ListAuditLog(r.Context(), limit, offset)
	if err != nil {
		s.logger.Error("list audit log", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeCollection(w, r, entries, int64(total), "")
}

// handleExportAuditLog exports the audit log in CSV or JSON format.
// GET /api/v1/audit-logs/export?format=csv|json
func (s *Server) handleExportAuditLog(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	limitParam := 10000
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limitParam = n
		}
	}

	entries, err := s.store.ExportAuditLog(r.Context(), limitParam)
	if err != nil {
		s.logger.Error("export audit log", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if entries == nil {
		entries = nil // keep nil for JSON null → handled below
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="audit-log.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"id", "user_id", "username", "action", "resource", "resource_id", "ip_address", "logged_at"})
		for _, e := range entries {
			uid := ""
			if e.UserID != nil {
				uid = *e.UserID
			}
			_ = cw.Write([]string{
				strconv.FormatInt(e.ID, 10),
				uid,
				e.Username,
				e.Action,
				e.Resource,
				e.ResourceID,
				e.IPAddress,
				e.LoggedAt.Format(time.RFC3339),
			})
		}
		cw.Flush()
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="audit-log.json"`)
		_ = json.NewEncoder(w).Encode(entries)
	}
}

// handleGetUserActivity returns audit log entries for a specific user.
// GET /api/v1/users/{id}/activity
func (s *Server) handleGetUserActivity(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")

	limit := 100
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	entries, total, err := s.store.ListAuditLogByUser(r.Context(), userID, limit, offset)
	if err != nil {
		s.logger.Error("list audit log by user", "err", err, "user_id", userID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeCollection(w, r, entries, int64(total), "")
}
