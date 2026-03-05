package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	agentcfg "github.com/badskater/distributed-encoder/internal/agent/config"
)

// upgradeChecker polls the controller HTTP API for a newer agent version.
type upgradeChecker struct {
	controllerHTTPBase string // e.g. "https://controller.example.com:8080"
	currentVersion     string // e.g. "0.1.0"
	log                *slog.Logger
}

// upgradeCheckResponse is the JSON returned by the controller upgrade check endpoint.
type upgradeCheckResponse struct {
	Version   string              `json:"version"`
	Available []upgradeArtifact   `json:"available"`
}

// upgradeArtifact describes a downloadable agent binary.
type upgradeArtifact struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	URL  string `json:"url"`
	SHA  string `json:"sha256"`
}

// newUpgradeChecker creates an upgradeChecker. controllerHTTPBase is derived
// from the gRPC address by substituting the HTTP port (from config).
func newUpgradeChecker(cfg *agentcfg.Config, log *slog.Logger) *upgradeChecker {
	scheme := "http"
	if cfg.Controller.TLS.Cert != "" {
		scheme = "https"
	}

	host := cfg.Controller.Address
	// Strip any port from the address and use 8080 for HTTP.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	base := fmt.Sprintf("%s://%s:8080", scheme, host)

	return &upgradeChecker{
		controllerHTTPBase: base,
		currentVersion:     agentVersion,
		log:                log,
	}
}

// checkLoop polls every 10 minutes. When a newer version is found and the
// agent is idle (isBusy func returns false), it calls applyUpgrade.
func (u *upgradeChecker) checkLoop(ctx context.Context, isBusy func() bool) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.check(ctx, isBusy)
		}
	}
}

// check performs a single upgrade check against the controller.
func (u *upgradeChecker) check(ctx context.Context, isBusy func() bool) {
	url := u.controllerHTTPBase + "/api/v1/agent/upgrade/check"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		u.log.Error("upgrade check: creating request", "error", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		u.log.Error("upgrade check: request failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		u.log.Warn("upgrade check: unexpected status", "status", resp.StatusCode)
		return
	}

	var checkResp upgradeCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&checkResp); err != nil {
		u.log.Error("upgrade check: decoding response", "error", err)
		return
	}

	if checkResp.Version == u.currentVersion {
		u.log.Debug("upgrade check: already up to date", "version", u.currentVersion)
		return
	}

	u.log.Info("upgrade available", "current", u.currentVersion, "new", checkResp.Version)

	// Find matching artifact for this OS/arch.
	var artifact *upgradeArtifact
	for i := range checkResp.Available {
		a := &checkResp.Available[i]
		if a.OS == runtime.GOOS && a.Arch == runtime.GOARCH {
			artifact = a
			break
		}
	}
	if artifact == nil {
		u.log.Warn("upgrade check: no artifact for this platform", "os", runtime.GOOS, "arch", runtime.GOARCH)
		return
	}

	if isBusy() {
		u.log.Info("upgrade deferred: agent is busy")
		return
	}

	downloadURL := u.controllerHTTPBase + artifact.URL
	if err := u.applyUpgrade(ctx, downloadURL, artifact.SHA); err != nil {
		u.log.Error("upgrade apply failed", "error", err)
	}
}

// applyUpgrade downloads the new binary, verifies its SHA-256 checksum,
// writes it to a staging path, and restarts the platform service.
func (u *upgradeChecker) applyUpgrade(ctx context.Context, downloadURL, expectedSHA256 string) error {
	u.log.Info("downloading upgrade", "url", downloadURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("creating download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Stream to a temp file.
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}
	dir := filepath.Dir(exePath)
	tmpFile, err := os.CreateTemp(dir, "agent-upgrade-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing download: %w", err)
	}
	tmpFile.Close()

	// Verify SHA-256 if provided.
	if expectedSHA256 != "" {
		actual := hex.EncodeToString(hasher.Sum(nil))
		if actual != expectedSHA256 {
			os.Remove(tmpPath)
			return fmt.Errorf("sha256 mismatch: expected %s, got %s", expectedSHA256, actual)
		}
		u.log.Info("sha256 verified", "hash", actual)
	}

	// Move to staging path.
	stagingPath := exePath + ".new"
	if err := os.Rename(tmpPath, stagingPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("staging binary: %w", err)
	}

	u.log.Info("upgrade downloaded, restart required", "staging_path", stagingPath)

	// Replace the executable.
	backupPath := exePath + ".old"
	os.Remove(backupPath) // clean up any previous backup
	if err := os.Rename(exePath, backupPath); err != nil {
		u.log.Error("backup current binary failed", "error", err)
		return fmt.Errorf("backing up current binary: %w", err)
	}
	if err := os.Rename(stagingPath, exePath); err != nil {
		// Attempt to restore from backup.
		_ = os.Rename(backupPath, exePath)
		return fmt.Errorf("replacing binary: %w", err)
	}

	// Best-effort service restart using the platform service manager.
	u.log.Info("attempting service restart")
	u.restartService(ctx)

	return nil
}

// restartService performs a best-effort service restart using the
// platform-appropriate service manager.
func (u *upgradeChecker) restartService(ctx context.Context) {
	switch runtime.GOOS {
	case "windows":
		if out, err := exec.CommandContext(ctx, "sc.exe", "stop", serviceName).CombinedOutput(); err != nil {
			u.log.Warn("sc stop failed (may be running in foreground)", "error", err, "output", string(out))
		}
		// Give the service a moment to stop.
		time.Sleep(2 * time.Second)
		if out, err := exec.CommandContext(ctx, "sc.exe", "start", serviceName).CombinedOutput(); err != nil {
			u.log.Warn("sc start failed", "error", err, "output", string(out))
		}
	case "linux":
		if out, err := exec.CommandContext(ctx, "systemctl", "restart", serviceName).CombinedOutput(); err != nil {
			u.log.Warn("systemctl restart failed (may be running in foreground)", "error", err, "output", string(out))
		}
	default:
		u.log.Warn("service restart not supported on this platform", "os", runtime.GOOS)
	}
}
