package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/badskater/encodeswarmr/internal/db"
)

// SubtitleTrack describes a single subtitle stream in a media file.
type SubtitleTrack struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Codec    string `json:"codec"`
	Title    string `json:"title"`
}

// handleGetSourceSubtitles probes the source file and returns all subtitle
// streams found.
//
// GET /api/v1/sources/{id}/subtitles
func (s *Server) handleGetSourceSubtitles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	source, err := s.store.GetSourceByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "source not found")
		return
	}
	if err != nil {
		s.logger.Error("get source for subtitles", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	localPath, err := s.translateSourcePath(r.Context(), source.UNCPath)
	if err != nil {
		s.logger.Warn("subtitle probe: path translate failed",
			"source_id", id, "err", err)
		// Fall back to using the original path.
		localPath = source.UNCPath
	}

	tracks, err := probeSubtitleTracks(r.Context(), s.ffprobeBin(), localPath)
	if err != nil {
		s.logger.Error("subtitle probe failed", "source_id", id, "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error",
			"subtitle probe failed: "+err.Error())
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"source_id": id,
		"tracks":    tracks,
	})
}

// handleGetSourceThumbnails returns the thumbnail URLs for a source.
//
// GET /api/v1/sources/{id}/thumbnails
func (s *Server) handleGetSourceThumbnails(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	source, err := s.store.GetSourceByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "source not found")
		return
	}
	if err != nil {
		s.logger.Error("get source for thumbnails", "err", err, "source_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Build absolute API URL paths from the relative thumbnail paths.
	urls := make([]string, 0, len(source.Thumbnails))
	for _, p := range source.Thumbnails {
		// p is stored as "sourceID/filename"; serve via /api/v1/thumbnails/{path}
		urls = append(urls, "/api/v1/thumbnails/"+p)
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"source_id":  id,
		"thumbnails": urls,
	})
}

// ffprobeBin returns the configured ffprobe binary path, defaulting to "ffprobe".
func (s *Server) ffprobeBin() string {
	if s.cfg != nil && s.cfg.Analysis.FFprobeBin != "" {
		return s.cfg.Analysis.FFprobeBin
	}
	return "ffprobe"
}

// translateSourcePath applies path mappings to sourcePath.  Returns the
// original path unchanged if no mapping applies or an error occurs.
func (s *Server) translateSourcePath(ctx context.Context, sourcePath string) (string, error) {
	mappings, err := s.store.ListPathMappings(ctx)
	if err != nil {
		return sourcePath, fmt.Errorf("list path mappings: %w", err)
	}
	// Re-use the same logic as the analysis runner.
	result := applyPathMappings(sourcePath, mappings)
	return result, nil
}

// applyPathMappings applies the first matching path mapping to sourcePath.
func applyPathMappings(sourcePath string, mappings []*db.PathMapping) string {
	isUNC := strings.HasPrefix(sourcePath, `\\`)
	for _, m := range mappings {
		if !m.Enabled {
			continue
		}
		if isUNC {
			if !strings.HasPrefix(strings.ToLower(sourcePath), strings.ToLower(m.WindowsPrefix)) {
				continue
			}
			remainder := sourcePath[len(m.WindowsPrefix):]
			remainder = strings.ReplaceAll(remainder, `\`, "/")
			linuxBase := strings.TrimRight(m.LinuxPrefix, "/")
			return linuxBase + "/" + strings.TrimLeft(remainder, "/")
		}
		if strings.HasPrefix(sourcePath, m.LinuxPrefix) {
			return sourcePath
		}
	}
	return sourcePath
}

// ---------------------------------------------------------------------------
// ffprobe subtitle probing
// ---------------------------------------------------------------------------

// ffprobeSubtitleOutput is the relevant portion of the ffprobe JSON output.
type ffprobeSubtitleOutput struct {
	Streams []struct {
		Index     int    `json:"index"`
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Tags      struct {
			Language string `json:"language"`
			Title    string `json:"title"`
		} `json:"tags"`
	} `json:"streams"`
}

// probeSubtitleTracks runs ffprobe and returns only the subtitle streams.
func probeSubtitleTracks(ctx context.Context, ffprobeBin, path string) ([]SubtitleTrack, error) {
	cmd := exec.CommandContext(ctx, ffprobeBin,
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=index,codec_name,codec_type:stream_tags=language,title",
		"-of", "json",
		path,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe: %w (stderr: %s)", err, stderr.String())
	}

	var out ffprobeSubtitleOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	tracks := make([]SubtitleTrack, 0, len(out.Streams))
	subIdx := 0
	for _, st := range out.Streams {
		if st.CodecType != "subtitle" {
			continue
		}
		tracks = append(tracks, SubtitleTrack{
			Index:    subIdx,
			Language: st.Tags.Language,
			Codec:    st.CodecName,
			Title:    st.Tags.Title,
		})
		subIdx++
	}
	return tracks, nil
}

// ---------------------------------------------------------------------------
// Thumbnail static file serving
// ---------------------------------------------------------------------------

// handleServeThumbnail serves a thumbnail image from the thumbnail directory.
// The path parameter contains the relative path after /api/v1/thumbnails/.
//
// GET /api/v1/thumbnails/{path...}
func (s *Server) handleServeThumbnail(w http.ResponseWriter, r *http.Request) {
	// Grab the remainder of the path after the prefix.
	rel := strings.TrimPrefix(r.URL.Path, "/api/v1/thumbnails/")
	rel = strings.TrimPrefix(rel, "/")

	if rel == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "thumbnail path required")
		return
	}

	// Security: reject any path that tries to escape the thumbnail directory.
	if strings.Contains(rel, "..") {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid thumbnail path")
		return
	}

	thumbDir := s.thumbnailDir()
	if thumbDir == "" {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "thumbnails not configured")
		return
	}

	// Validate that the first path segment is a valid UUID (source ID).
	parts := strings.SplitN(rel, "/", 2)
	if len(parts) != 2 || !isValidUUID(parts[0]) {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid thumbnail path")
		return
	}

	http.ServeFile(w, r, thumbDir+"/"+rel)
}

// thumbnailDir returns the configured thumbnail base directory from cfg.
func (s *Server) thumbnailDir() string {
	if s.cfg != nil {
		return s.cfg.Analysis.ThumbnailDir
	}
	return ""
}

// isValidUUID performs a lightweight check that s looks like a UUID v4.
func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !isHexChar(c) {
			return false
		}
	}
	return true
}

func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

