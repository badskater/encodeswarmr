package api

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	webui "github.com/badskater/encodeswarmr/web"
)

// staticHandler serves the embedded SPA. For any path that does not map
// to an actual file in dist/, it falls back to serving index.html so that
// client-side routing works correctly.
func (s *Server) staticHandler() (http.Handler, error) {
	dist, err := fs.Sub(webui.Files, "dist")
	if err != nil {
		return nil, fmt.Errorf("static: sub fs: %w", err)
	}
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to find the file in the embedded FS.
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean == "" {
			clean = "index.html"
		}
		f, err := dist.Open(clean)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// File not found — serve index.html for SPA client-side routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}), nil
}
