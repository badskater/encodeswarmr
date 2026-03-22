package api

import (
	"net/http"

	"github.com/badskater/encodeswarmr/internal/controller/presets"
)

// handleListPresets returns all built-in encoding presets.
func (s *Server) handleListPresets(w http.ResponseWriter, r *http.Request) {
	all := presets.All()
	writeCollection(w, r, all, int64(len(all)), "")
}

// handleGetPreset returns a single preset by name.
func (s *Server) handleGetPreset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	p := presets.Get(name)
	if p == nil {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "preset not found")
		return
	}
	writeJSON(w, r, http.StatusOK, p)
}

// handleListAudioPresets returns all built-in audio encoding presets.
//
// GET /api/v1/presets/audio
func (s *Server) handleListAudioPresets(w http.ResponseWriter, r *http.Request) {
	all := presets.AllAudio()
	writeCollection(w, r, all, int64(len(all)), "")
}
