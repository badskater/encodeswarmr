package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// safeNameRe restricts os/arch values to lowercase alphanumeric characters only.
var safeNameRe = regexp.MustCompile(`^[a-z0-9]+$`)

// handleAgentUpgradeCheck returns the current agent version and available binaries.
// GET /api/v1/agent/upgrade/check
func (s *Server) handleAgentUpgradeCheck(w http.ResponseWriter, r *http.Request) {
	version := s.cfg.Upgrade.Version
	if version == "" {
		version = "0.0.0"
	}

	available := listAvailableBinaries(s.cfg.Upgrade.BinDir)

	writeJSON(w, r, http.StatusOK, map[string]any{
		"version":   version,
		"available": available,
	})
}

// handleAgentUpgradeDownload streams an agent binary for the given os/arch.
// GET /api/v1/agent/upgrade/{os}/{arch}
func (s *Server) handleAgentUpgradeDownload(w http.ResponseWriter, r *http.Request) {
	targetOS := r.PathValue("os")
	targetArch := r.PathValue("arch")

	if !safeNameRe.MatchString(targetOS) || !safeNameRe.MatchString(targetArch) {
		writeProblem(w, r, http.StatusBadRequest, "Invalid parameters", "os and arch must be lowercase alphanumeric")
		return
	}

	filename := fmt.Sprintf("agent-%s-%s", targetOS, targetArch)
	if targetOS == "windows" {
		filename += ".exe"
	}

	fullPath := filepath.Join(s.cfg.Upgrade.BinDir, filename)

	f, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeProblem(w, r, http.StatusNotFound, "Binary not found",
				fmt.Sprintf("no binary available for %s/%s", targetOS, targetArch))
			return
		}
		s.logger.Error("failed to open agent binary", "path", fullPath, "error", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal error", "failed to read binary file")
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		s.logger.Error("failed to stat agent binary", "path", fullPath, "error", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal error", "failed to stat binary file")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, filename, info.ModTime(), f)
}

// listAvailableBinaries scans the directory for files matching the agent-{os}-{arch}
// naming convention and returns their metadata, including SHA-256 hashes.
func listAvailableBinaries(dir string) []map[string]string {
	result := []map[string]string{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "agent-") {
			continue
		}

		// Strip "agent-" prefix and optional ".exe" suffix.
		base := strings.TrimPrefix(name, "agent-")
		base = strings.TrimSuffix(base, ".exe")

		parts := strings.SplitN(base, "-", 2)
		if len(parts) != 2 {
			continue
		}
		osName := parts[0]
		archName := parts[1]

		if !safeNameRe.MatchString(osName) || !safeNameRe.MatchString(archName) {
			continue
		}

		sha := computeFileSHA256(filepath.Join(dir, name))

		result = append(result, map[string]string{
			"os":     osName,
			"arch":   archName,
			"url":    fmt.Sprintf("/api/v1/agent/upgrade/%s/%s", osName, archName),
			"sha256": sha,
		})
	}

	return result
}

// computeFileSHA256 reads the file at path and returns its SHA-256 hex digest.
// Returns an empty string if the file cannot be read.
func computeFileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}
