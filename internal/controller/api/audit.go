package api

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/db"
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

// handleExportAuditLog streams the filtered audit log as CSV or JSON.
// GET /api/v1/audit-logs/export?format=csv|json&from=...&to=...&user_id=...&action=...
func (s *Server) handleExportAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	format := q.Get("format")
	if format != "csv" && format != "json" {
		format = "json"
	}

	f := db.AuditLogFilter{}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			f.From = t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = t
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			// Include the full day.
			f.To = t.Add(24*time.Hour - time.Second)
		}
	}
	f.UserID = q.Get("user_id")
	f.Action = q.Get("action")

	entries, err := s.store.ExportAuditLog(r.Context(), f)
	if err != nil {
		s.logger.Error("export audit log", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if entries == nil {
		entries = []*db.AuditEntry{}
	}

	switch format {
	case "csv":
		var buf bytes.Buffer
		cw := csv.NewWriter(&buf)
		_ = cw.Write([]string{"id", "user_id", "username", "action", "resource", "resource_id", "ip_address", "logged_at"})
		for _, e := range entries {
			userID := ""
			if e.UserID != nil {
				userID = *e.UserID
			}
			_ = cw.Write([]string{
				strconv.FormatInt(e.ID, 10),
				userID,
				e.Username,
				e.Action,
				e.Resource,
				e.ResourceID,
				e.IPAddress,
				e.LoggedAt.UTC().Format(time.RFC3339),
			})
		}
		cw.Flush()
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="audit-log.csv"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="audit-log.json"`)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(entries)
	}
}

// handleAuditLogStats returns summary statistics for the audit log.
// GET /api/v1/audit-logs/stats
func (s *Server) handleAuditLogStats(w http.ResponseWriter, r *http.Request) {
	total, perAction, err := s.store.GetAuditLogStats(r.Context())
	if err != nil {
		s.logger.Error("audit log stats", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if perAction == nil {
		perAction = []*db.AuditActionStat{}
	}
	writeJSON(w, r, http.StatusOK, map[string]any{
		"total":      total,
		"per_action": perAction,
	})
}

// handleUserActivity returns the paginated audit trail for a specific user.
// GET /api/v1/users/{id}/activity
func (s *Server) handleUserActivity(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing user id")
		return
	}

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

	entries, total, err := s.store.ListUserAuditLog(r.Context(), userID, limit, offset)
	if err != nil {
		s.logger.Error("list user audit log", "err", err, "user_id", userID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeCollection(w, r, entries, int64(total), "")
}

// handleAnonymizeUserData exports a user's personal data as JSON then
// anonymises their audit log entries (GDPR right-to-erasure helper).
// DELETE /api/v1/users/{id}/data
func (s *Server) handleAnonymizeUserData(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing user id")
		return
	}

	// Fetch user data before anonymisation.
	user, err := s.store.GetUserByID(r.Context(), userID)
	if err != nil {
		s.logger.Error("get user for anonymization", "err", err, "user_id", userID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	auditEntries, _, err := s.store.ListUserAuditLog(r.Context(), userID, 500, 0)
	if err != nil {
		s.logger.Error("list user audit log for anonymization", "err", err, "user_id", userID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Anonymise audit log.
	if err := s.store.AnonymizeUserAuditLog(r.Context(), userID); err != nil {
		s.logger.Error("anonymize user audit log", "err", err, "user_id", userID)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Emit audit entry for the erasure itself — best-effort.
	erasureParams := db.CreateAuditEntryParams{
		Action:     "user.data_erased",
		Resource:   "user",
		ResourceID: userID,
		IPAddress:  r.RemoteAddr,
	}
	if claims, ok := auth.FromContext(r.Context()); ok {
		erasureParams.UserID = &claims.UserID
		erasureParams.Username = claims.Username
	}
	if err := s.store.CreateAuditEntry(r.Context(), erasureParams); err != nil {
		s.logger.Warn("audit log: anonymize user data", "err", err, "user_id", userID)
	}

	// Return the exported data so the caller can download it.
	export := map[string]any{
		"user": map[string]any{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"role":       user.Role,
			"created_at": user.CreatedAt,
		},
		"audit_entries":  auditEntries,
		"exported_at":    fmt.Sprintf("%s", time.Now().UTC().Format(time.RFC3339)),
		"note":           "Audit log entries have been anonymised. This export is the last record linking this user to their actions.",
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="user-data-%s.json"`, userID))
	writeJSON(w, r, http.StatusOK, export)
}
