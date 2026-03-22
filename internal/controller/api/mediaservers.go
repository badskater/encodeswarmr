package api

import (
	"net/http"
)

// handleListMediaServers returns the list of configured media server integrations.
//
// GET /api/v1/media-servers
func (s *Server) handleListMediaServers(w http.ResponseWriter, r *http.Request) {
	servers := s.mediaManager.Servers()
	infos := make([]mediaServerInfo, 0, len(servers))
	for i, srv := range servers {
		cfg := s.cfg.MediaServers[i]
		infos = append(infos, mediaServerInfo{
			Name:        srv.Name(),
			Type:        srv.Type(),
			AutoRefresh: cfg.AutoRefresh,
		})
	}
	writeCollection(w, r, infos, int64(len(infos)), "")
}

// handleRefreshMediaServer manually triggers a library refresh on the named
// media server.
//
// POST /api/v1/media-servers/{name}/refresh
func (s *Server) handleRefreshMediaServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	srv, err := s.mediaManager.GetByName(name)
	if err != nil {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "media server not found")
		return
	}

	if err := srv.RefreshLibrary(r.Context()); err != nil {
		s.logger.Error("manual media server refresh failed", "name", name, "err", err)
		writeProblem(w, r, http.StatusBadGateway, "Bad Gateway", "media server refresh failed: "+err.Error())
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true, "name": name})
}

// mediaServerInfo is the API response shape for a single media server entry.
type mediaServerInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	AutoRefresh bool   `json:"auto_refresh"`
}

