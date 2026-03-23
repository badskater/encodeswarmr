package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Config access
// ---------------------------------------------------------------------------

// allowedPaths returns the configured list of base paths the file manager may
// access.  An empty list means no access is granted.
func (s *Server) allowedPaths() []string {
	if s.cfg == nil {
		return nil
	}
	return s.cfg.FileManager.AllowedPaths
}

// isPathAllowed returns true when path is inside (or equal to) one of the
// allowed base paths.  Symlink traversal is not resolved — callers are
// expected to pass Clean paths.
func isPathAllowed(path string, allowed []string) bool {
	clean := filepath.Clean(path)
	for _, base := range allowed {
		base = filepath.Clean(base)
		if clean == base || strings.HasPrefix(clean, base+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// File entry types
// ---------------------------------------------------------------------------

// FileEntry describes a file or directory returned by the browse endpoint.
type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	// Ext is the file extension, lower-cased (e.g. ".mkv").
	Ext string `json:"ext,omitempty"`
	// IsVideo is true when Ext is a known video container.
	IsVideo bool `json:"is_video,omitempty"`
}

var videoExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".mov": true, ".avi": true,
	".ts": true, ".m2ts": true, ".mts": true, ".wmv": true,
	".flv": true, ".webm": true, ".hevc": true, ".m4v": true,
}

// ---------------------------------------------------------------------------
// Browse
// ---------------------------------------------------------------------------

// handleBrowseFiles lists the contents of a directory.
//
// GET /api/v1/files/browse?path=/mnt/nas/media
func (s *Server) handleBrowseFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "path query parameter is required")
		return
	}

	path = filepath.Clean(path)
	if !isPathAllowed(path, s.allowedPaths()) {
		writeProblem(w, r, http.StatusForbidden, "Forbidden", "path is outside allowed directories")
		return
	}

	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "directory not found")
		return
	}
	if err != nil {
		s.logger.Error("file manager browse", "path", path, "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	files := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		fe := FileEntry{
			Name:    e.Name(),
			Path:    filepath.Join(path, e.Name()),
			IsDir:   e.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Ext:     ext,
			IsVideo: videoExtensions[ext],
		}
		files = append(files, fe)
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"path":    path,
		"entries": files,
	})
}

// ---------------------------------------------------------------------------
// File info
// ---------------------------------------------------------------------------

// FileInfo extends FileEntry with optional codec information.
type FileInfo struct {
	FileEntry
	// CodecInfo holds ffprobe output for video files; nil for non-video.
	CodecInfo json.RawMessage `json:"codec_info,omitempty"`
}

// handleFileInfo returns metadata for a single file, including ffprobe data
// for video files.
//
// GET /api/v1/files/info?path=/mnt/nas/media/movie.mkv
func (s *Server) handleFileInfo(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "path query parameter is required")
		return
	}
	path = filepath.Clean(path)
	if !isPathAllowed(path, s.allowedPaths()) {
		writeProblem(w, r, http.StatusForbidden, "Forbidden", "path is outside allowed directories")
		return
	}

	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "file not found")
		return
	}
	if err != nil {
		s.logger.Error("file manager info stat", "path", path, "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	ext := strings.ToLower(filepath.Ext(path))
	fi := FileInfo{
		FileEntry: FileEntry{
			Name:    filepath.Base(path),
			Path:    path,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Ext:     ext,
			IsVideo: videoExtensions[ext],
		},
	}

	// Probe codec info for video files.
	if fi.IsVideo && !fi.IsDir {
		probeData, probeErr := probeFileInfo(r.Context(), s.ffprobeBin(), path)
		if probeErr != nil {
			s.logger.Warn("file manager ffprobe", "path", path, "err", probeErr)
		} else {
			fi.CodecInfo = probeData
		}
	}

	writeJSON(w, r, http.StatusOK, fi)
}

// probeFileInfo runs ffprobe and returns the raw JSON output.
func probeFileInfo(ctx context.Context, ffprobeBin, path string) (json.RawMessage, error) {
	cmd := exec.CommandContext(ctx, ffprobeBin,
		"-v", "error",
		"-show_streams", "-show_format",
		"-of", "json",
		path,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe: %w (stderr: %s)", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// ---------------------------------------------------------------------------
// Move
// ---------------------------------------------------------------------------

// handleMoveFile moves a file or directory to a new location.
//
// POST /api/v1/files/move
// Body: {"source": "/path/a", "destination": "/path/b"}
func (s *Server) handleMoveFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Source == "" || req.Destination == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "source and destination are required")
		return
	}

	src := filepath.Clean(req.Source)
	dst := filepath.Clean(req.Destination)

	if !isPathAllowed(src, s.allowedPaths()) {
		writeProblem(w, r, http.StatusForbidden, "Forbidden", "source path is outside allowed directories")
		return
	}
	if !isPathAllowed(dst, s.allowedPaths()) {
		writeProblem(w, r, http.StatusForbidden, "Forbidden", "destination path is outside allowed directories")
		return
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		s.logger.Error("file manager move mkdir", "dst", dst, "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	if err := os.Rename(src, dst); err != nil {
		s.logger.Error("file manager move", "src", src, "dst", dst, "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error",
			"move failed: "+err.Error())
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"source":      src,
		"destination": dst,
		"moved":       true,
	})
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

// handleDeleteFile deletes a single file (not a directory).
//
// DELETE /api/v1/files/{path...}
func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		// Also accept path as a trailing wildcard segment.
		path = strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	}
	if path == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "path is required")
		return
	}

	path = filepath.Clean("/" + path)
	if !isPathAllowed(path, s.allowedPaths()) {
		writeProblem(w, r, http.StatusForbidden, "Forbidden", "path is outside allowed directories")
		return
	}

	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "file not found")
		return
	}
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if info.IsDir() {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "cannot delete a directory via this endpoint")
		return
	}

	if err := os.Remove(path); err != nil {
		s.logger.Error("file manager delete", "path", path, "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error",
			"delete failed: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Download
// ---------------------------------------------------------------------------

// handleDownloadFile streams a file to the client.
//
// GET /api/v1/files/download?path=/mnt/nas/output/movie.mkv
func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "path query parameter is required")
		return
	}
	path = filepath.Clean(path)
	if !isPathAllowed(path, s.allowedPaths()) {
		writeProblem(w, r, http.StatusForbidden, "Forbidden", "path is outside allowed directories")
		return
	}

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "file not found")
		return
	}
	if err != nil {
		s.logger.Error("file manager download open", "path", path, "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err == nil && info.IsDir() {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "cannot download a directory")
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	if err == nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	}

	if _, err := io.Copy(w, f); err != nil {
		s.logger.Warn("file manager download copy", "path", path, "err", err)
	}
}
