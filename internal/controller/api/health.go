package api

import "net/http"

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{"status": "ok"})
}
