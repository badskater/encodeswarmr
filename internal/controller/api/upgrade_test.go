package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/controller/webhooks"
)

// newTestServerWithCfg creates a test server with a supplied config (for
// handlers that dereference s.cfg, such as the upgrade endpoints).
func newTestServerWithCfg(_ any, cfg *config.Config) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wh := webhooks.New(&stubStore{}, webhooks.Config{}, logger)
	return &Server{
		store:    &stubStore{},
		logger:   logger,
		webhooks: wh,
		cfg:      cfg,
	}
}

// ---------------------------------------------------------------------------
// handleAgentUpgradeCheck
// ---------------------------------------------------------------------------

func TestHandleAgentUpgradeCheck_EmptyBinDir(t *testing.T) {
	cfg := &config.Config{}
	cfg.Upgrade.Version = "1.2.3"
	cfg.Upgrade.BinDir = t.TempDir() // empty directory — no binaries present

	srv := newTestServerWithCfg(nil, cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/upgrade/check", nil)
	srv.handleAgentUpgradeCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		Data struct {
			Version   string                   `json:"version"`
			Available []map[string]interface{} `json:"available"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Version != "1.2.3" {
		t.Errorf("version = %q, want %q", body.Data.Version, "1.2.3")
	}
	if len(body.Data.Available) != 0 {
		t.Errorf("available = %d, want 0", len(body.Data.Available))
	}
}

func TestHandleAgentUpgradeCheck_DefaultVersion(t *testing.T) {
	cfg := &config.Config{}
	cfg.Upgrade.BinDir = t.TempDir()
	// Version is intentionally left empty — should default to "0.0.0".

	srv := newTestServerWithCfg(nil, cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/upgrade/check", nil)
	srv.handleAgentUpgradeCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Version != "0.0.0" {
		t.Errorf("version = %q, want %q", body.Data.Version, "0.0.0")
	}
}

func TestHandleAgentUpgradeCheck_WithBinaries(t *testing.T) {
	dir := t.TempDir()

	// Create fake agent binary files.
	for _, name := range []string{"agent-linux-amd64", "agent-windows-amd64.exe"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fake"), 0o644); err != nil {
			t.Fatalf("create binary: %v", err)
		}
	}

	cfg := &config.Config{}
	cfg.Upgrade.Version = "2.0.0"
	cfg.Upgrade.BinDir = dir

	srv := newTestServerWithCfg(nil, cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/upgrade/check", nil)
	srv.handleAgentUpgradeCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		Data struct {
			Available []map[string]interface{} `json:"available"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data.Available) != 2 {
		t.Errorf("available count = %d, want 2", len(body.Data.Available))
	}
}

// ---------------------------------------------------------------------------
// handleAgentUpgradeDownload
// ---------------------------------------------------------------------------

func TestHandleAgentUpgradeDownload_InvalidOSArch(t *testing.T) {
	cases := []struct {
		os   string
		arch string
	}{
		{"linux", "amd64/../evil"},
		{"win.dows", "amd64"},
		{"linux", ""},
		{"", "amd64"},
		{"LINUX", "AMD64"}, // uppercase not allowed by safeNameRe
	}

	cfg := &config.Config{}
	cfg.Upgrade.BinDir = t.TempDir()
	srv := newTestServerWithCfg(nil, cfg)

	for _, tc := range cases {
		t.Run(tc.os+"/"+tc.arch, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/upgrade/"+tc.os+"/"+tc.arch, nil)
			req.SetPathValue("os", tc.os)
			req.SetPathValue("arch", tc.arch)
			srv.handleAgentUpgradeDownload(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("os=%q arch=%q: status = %d, want %d", tc.os, tc.arch, rr.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandleAgentUpgradeDownload_NotFound(t *testing.T) {
	cfg := &config.Config{}
	cfg.Upgrade.BinDir = t.TempDir() // empty — no binary present

	srv := newTestServerWithCfg(nil, cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/upgrade/linux/amd64", nil)
	req.SetPathValue("os", "linux")
	req.SetPathValue("arch", "amd64")
	srv.handleAgentUpgradeDownload(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleAgentUpgradeDownload_LinuxSuccess(t *testing.T) {
	dir := t.TempDir()
	content := []byte("fake-linux-binary")
	if err := os.WriteFile(filepath.Join(dir, "agent-linux-amd64"), content, 0o644); err != nil {
		t.Fatalf("create binary: %v", err)
	}

	cfg := &config.Config{}
	cfg.Upgrade.BinDir = dir

	srv := newTestServerWithCfg(nil, cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/upgrade/linux/amd64", nil)
	req.SetPathValue("os", "linux")
	req.SetPathValue("arch", "amd64")
	srv.handleAgentUpgradeDownload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	disp := rr.Header().Get("Content-Disposition")
	if disp == "" {
		t.Error("Content-Disposition header not set")
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/octet-stream")
	}
}

func TestHandleAgentUpgradeDownload_WindowsBinaryName(t *testing.T) {
	dir := t.TempDir()
	content := []byte("fake-windows-binary")
	// Windows binary must have .exe suffix.
	if err := os.WriteFile(filepath.Join(dir, "agent-windows-amd64.exe"), content, 0o644); err != nil {
		t.Fatalf("create binary: %v", err)
	}

	cfg := &config.Config{}
	cfg.Upgrade.BinDir = dir

	srv := newTestServerWithCfg(nil, cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/upgrade/windows/amd64", nil)
	req.SetPathValue("os", "windows")
	req.SetPathValue("arch", "amd64")
	srv.handleAgentUpgradeDownload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// ---------------------------------------------------------------------------
// listAvailableBinaries (internal helper)
// ---------------------------------------------------------------------------

func TestListAvailableBinaries_NonexistentDir(t *testing.T) {
	result := listAvailableBinaries("/nonexistent/path/xyz")
	if len(result) != 0 {
		t.Errorf("expected empty slice for nonexistent dir, got %d entries", len(result))
	}
}

func TestListAvailableBinaries_IgnoresNonAgentFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"README.md", "config.yaml", "not-agent-linux-amd64"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("create file: %v", err)
		}
	}
	result := listAvailableBinaries(dir)
	if len(result) != 0 {
		t.Errorf("expected 0 agent binaries, got %d", len(result))
	}
}

func TestListAvailableBinaries_IgnoresInvalidNames(t *testing.T) {
	dir := t.TempDir()
	// File that starts with "agent-" but doesn't have valid os-arch split.
	for _, name := range []string{"agent-noarch", "agent-"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("create file: %v", err)
		}
	}
	result := listAvailableBinaries(dir)
	if len(result) != 0 {
		t.Errorf("expected 0 valid agent binaries, got %d", len(result))
	}
}

func TestListAvailableBinaries_SHA256Present(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agent-linux-amd64"), []byte("data"), 0o644); err != nil {
		t.Fatalf("create binary: %v", err)
	}
	result := listAvailableBinaries(dir)
	if len(result) != 1 {
		t.Fatalf("expected 1 binary, got %d", len(result))
	}
	sha := result[0]["sha256"]
	if sha == "" {
		t.Error("sha256 should not be empty")
	}
}
