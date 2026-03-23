package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/badskater/encodeswarmr/internal/db"
)

// safeNameRe restricts os/arch/channel values to lowercase alphanumeric characters only.
var safeNameRe = regexp.MustCompile(`^[a-z0-9]+$`)

// validChannels is the set of accepted release channel names.
var validChannels = map[string]bool{
	"stable":  true,
	"beta":    true,
	"nightly": true,
}

// handleAgentUpgradeCheck returns the current agent version and available binaries.
// If the agent_id query parameter is present and that agent has upgrade_requested=true,
// the response will include push_requested=true and the flag will be cleared.
//
// The optional channel query parameter restricts results to a specific release channel.
// When channel is omitted the agent's configured channel (or "stable") is used.
//
// GET /api/v1/agent/upgrade/check
func (s *Server) handleAgentUpgradeCheck(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query().Get("channel")
	if channel == "" {
		channel = "stable"
	}
	if !validChannels[channel] {
		channel = "stable"
	}

	// Determine version: prefer DB record for the channel, fall back to config.
	version := s.resolveChannelVersion(r, channel)

	available := listAvailableBinaries(s.cfg.Upgrade.BinDir)

	resp := map[string]any{
		"version":   version,
		"channel":   channel,
		"available": available,
	}

	// If the agent identifies itself, check and clear the push upgrade flag.
	if agentID := r.URL.Query().Get("agent_id"); agentID != "" {
		agent, err := s.store.GetAgentByID(r.Context(), agentID)
		if err != nil && !errors.Is(err, db.ErrNotFound) {
			s.logger.Error("upgrade check: get agent", "err", err, "agent_id", agentID)
		} else if err == nil && agent.UpgradeRequested {
			resp["push_requested"] = true
			// Clear the flag — best-effort, do not fail the response.
			if clearErr := s.store.ClearAgentUpgradeRequested(r.Context(), agentID); clearErr != nil {
				s.logger.Warn("upgrade check: clear upgrade_requested", "err", clearErr, "agent_id", agentID)
			}
		}
	}

	writeJSON(w, r, http.StatusOK, resp)
}

// resolveChannelVersion returns the version string for the given channel.
// It consults the upgrade_binaries table first; if no record exists it falls
// back to the global s.cfg.Upgrade.Version.
func (s *Server) resolveChannelVersion(r *http.Request, channel string) string {
	binaries, err := s.store.ListUpgradeBinaries(r.Context(), channel)
	if err == nil && len(binaries) > 0 {
		return binaries[0].Version
	}
	v := s.cfg.Upgrade.Version
	if v == "" {
		return "0.0.0"
	}
	return v
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

// handleListUpgradeChannels lists available release channels with the latest version per channel.
// GET /api/v1/upgrade-channels
func (s *Server) handleListUpgradeChannels(w http.ResponseWriter, r *http.Request) {
	binaries, err := s.store.ListUpgradeBinaries(r.Context(), "")
	if err != nil {
		s.logger.Error("list upgrade channels", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Build a per-channel summary: latest version + available os/arch combinations.
	type channelInfo struct {
		Channel  string   `json:"channel"`
		Version  string   `json:"version"`
		Binaries []string `json:"binaries"` // "os/arch"
	}
	byChannel := map[string]*channelInfo{}
	for _, b := range binaries {
		ci, ok := byChannel[b.Channel]
		if !ok {
			ci = &channelInfo{Channel: b.Channel, Version: b.Version}
			byChannel[b.Channel] = ci
		}
		ci.Binaries = append(ci.Binaries, b.OS+"/"+b.Arch)
	}

	// Also include channels that only have filesystem binaries (legacy layout).
	diskBinaries := listAvailableBinaries(s.cfg.Upgrade.BinDir)
	if len(diskBinaries) > 0 {
		if _, ok := byChannel["stable"]; !ok {
			byChannel["stable"] = &channelInfo{
				Channel:  "stable",
				Version:  s.cfg.Upgrade.Version,
				Binaries: []string{},
			}
		}
		for _, b := range diskBinaries {
			osPart, _ := b["os"], b["arch"]
			archPart := b["arch"]
			entry := osPart + "/" + archPart
			ci := byChannel["stable"]
			found := false
			for _, existing := range ci.Binaries {
				if existing == entry {
					found = true
					break
				}
			}
			if !found {
				ci.Binaries = append(ci.Binaries, entry)
			}
		}
	}

	// Return ordered slice.
	result := make([]*channelInfo, 0, len(byChannel))
	for _, ci := range byChannel {
		result = append(result, ci)
	}
	writeJSON(w, r, http.StatusOK, result)
}

// handleUploadAgentBinary accepts a multipart binary upload with channel/version metadata.
// POST /api/v1/upgrades/upload
func (s *Server) handleUploadAgentBinary(w http.ResponseWriter, r *http.Request) {
	// Parse metadata from JSON body (channel, version, os, arch).
	var req struct {
		Channel string `json:"channel"`
		Version string `json:"version"`
		OS      string `json:"os"`
		Arch    string `json:"arch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if !validChannels[req.Channel] {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "channel must be stable, beta, or nightly")
		return
	}
	if req.Version == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "version is required")
		return
	}
	if !safeNameRe.MatchString(req.OS) || !safeNameRe.MatchString(req.Arch) {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "os and arch must be lowercase alphanumeric")
		return
	}

	// Record metadata in the DB (no file upload in this endpoint — files are
	// placed on the controller filesystem directly or via separate deployment).
	filename := fmt.Sprintf("agent-%s-%s", req.OS, req.Arch)
	if req.OS == "windows" {
		filename += ".exe"
	}
	fullPath := filepath.Join(s.cfg.Upgrade.BinDir, filename)
	sha := computeFileSHA256(fullPath)

	rec, err := s.store.UpsertUpgradeBinary(r.Context(), db.UpsertUpgradeBinaryParams{
		Channel:  req.Channel,
		Version:  req.Version,
		OS:       req.OS,
		Arch:     req.Arch,
		Filename: filename,
		SHA256:   sha,
	})
	if err != nil {
		s.logger.Error("upsert upgrade binary", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, rec)
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
