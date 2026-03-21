package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/badskater/distributed-encoder/internal/controller/plugins"
)

// handleListPlugins returns all registered plugins.
// Requires viewer role.
func (s *Server) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	if s.plugins == nil {
		writeJSON(w, r, http.StatusOK, []*plugins.Plugin{})
		return
	}
	list := s.plugins.List()
	writeJSON(w, r, http.StatusOK, list)
}

// handleEnablePlugin marks the named plugin as enabled.
// Requires admin role.
func (s *Server) handleEnablePlugin(w http.ResponseWriter, r *http.Request) {
	s.setPluginEnabled(w, r, true)
}

// handleDisablePlugin marks the named plugin as disabled.
// Requires admin role.
func (s *Server) handleDisablePlugin(w http.ResponseWriter, r *http.Request) {
	s.setPluginEnabled(w, r, false)
}

func (s *Server) setPluginEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	name := r.PathValue("name")
	if name == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "plugin name is required")
		return
	}

	if s.plugins == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "Service Unavailable", "plugin registry is not initialised")
		return
	}

	var err error
	if enabled {
		err = s.plugins.Enable(name)
	} else {
		err = s.plugins.Disable(name)
	}
	if err != nil {
		if errors.Is(err, plugins.ErrPluginNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", fmt.Sprintf("plugin %q not found", name))
			return
		}
		s.logger.Error("set plugin enabled", "name", name, "enabled", enabled, "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	p := s.plugins.Get(name)
	writeJSON(w, r, http.StatusOK, p)
}

