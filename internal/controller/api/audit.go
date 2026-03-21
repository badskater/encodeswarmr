package api

import (
	"net/http"
	"strconv"
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
