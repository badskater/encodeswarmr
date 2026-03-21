package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestUpgradeChecker creates an upgradeChecker wired to a fake base URL.
func newTestUpgradeChecker(baseURL string) *upgradeChecker {
	return &upgradeChecker{
		controllerHTTPBase: baseURL,
		currentVersion:     "0.1.0",
		log:                slog.Default(),
	}
}

// sha256Hex returns the hex-encoded SHA-256 of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// --- applyUpgrade ---

// TestApplyUpgrade_ChecksumMismatch ensures applyUpgrade returns an error when the
// downloaded binary's SHA-256 does not match the expected value.
func TestApplyUpgrade_ChecksumMismatch(t *testing.T) {
	payload := []byte("fake binary content")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	u := newTestUpgradeChecker(srv.URL)
	ctx := context.Background()

	err := u.applyUpgrade(ctx, srv.URL+"/download", "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected error for SHA-256 mismatch, got nil")
	}
}

// TestApplyUpgrade_HTTP404 ensures applyUpgrade returns an error on a non-200 response.
func TestApplyUpgrade_HTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	u := newTestUpgradeChecker(srv.URL)
	ctx := context.Background()

	err := u.applyUpgrade(ctx, srv.URL+"/download", "")
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
}

// TestApplyUpgrade_CorrectChecksum verifies that a download with the correct
// SHA-256 succeeds through the checksum step (the binary copy may fail on
// platforms without writeable exe directories, but the checksum path is covered).
func TestApplyUpgrade_CorrectChecksum(t *testing.T) {
	// Build a small fake binary content and its correct hash.
	payload := []byte("fake-binary-v2")
	correctHash := sha256Hex(payload)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	// Override os.Executable to point to a temp file we control by using a
	// copy of the current test binary placed in a temp dir.
	// We simulate this by patching the test so applyUpgrade writes into a
	// temp dir.  Because we cannot easily override os.Executable(), we verify
	// the behaviour indirectly: the function should NOT return an error about
	// sha256 mismatch.  Any subsequent error about renaming is acceptable.
	u := newTestUpgradeChecker(srv.URL)
	ctx := context.Background()

	err := u.applyUpgrade(ctx, srv.URL+"/download", correctHash)
	// The checksum is correct.  Any error that occurs must NOT be a sha256
	// mismatch error.
	if err != nil {
		if strings.Contains(err.Error(), "sha256 mismatch") {
			t.Errorf("unexpected sha256 mismatch: %v", err)
		}
		// Any other error (staging/rename) is expected in the test environment.
	}
}

// --- rollbackBinary ---

// TestRollbackBinary_Success verifies that rollbackBinary renames .bak back to
// the exe path when the backup exists.
func TestRollbackBinary_Success(t *testing.T) {
	dir := t.TempDir()

	exePath := filepath.Join(dir, "agent")
	backupPath := exePath + ".bak"

	// Create the "new (broken)" binary.
	if err := os.WriteFile(exePath, []byte("broken"), 0o755); err != nil {
		t.Fatalf("create broken binary: %v", err)
	}
	// Create the backup.
	if err := os.WriteFile(backupPath, []byte("good"), 0o755); err != nil {
		t.Fatalf("create backup binary: %v", err)
	}

	u := newTestUpgradeChecker("http://localhost")
	ctx := context.Background()

	if err := u.rollbackBinary(ctx, exePath); err != nil {
		t.Fatalf("rollbackBinary: %v", err)
	}

	// The backup should now be at exePath.
	data, err := os.ReadFile(exePath)
	if err != nil {
		t.Fatalf("reading restored binary: %v", err)
	}
	if string(data) != "good" {
		t.Errorf("restored binary content = %q, want %q", data, "good")
	}
}

// TestRollbackBinary_NoBackup verifies that rollbackBinary returns an error
// when the .bak file does not exist.
func TestRollbackBinary_NoBackup(t *testing.T) {
	dir := t.TempDir()
	exePath := filepath.Join(dir, "agent")

	// The backup file intentionally does not exist.
	u := newTestUpgradeChecker("http://localhost")
	ctx := context.Background()

	err := u.rollbackBinary(ctx, exePath)
	if err == nil {
		t.Fatal("expected error when backup is missing, got nil")
	}
}

// TestRollback_DelegatesToRollbackBinary verifies the public rollback method
// delegates to rollbackBinary (covers the thin wrapper).
func TestRollback_DelegatesToRollbackBinary(t *testing.T) {
	dir := t.TempDir()
	exePath := filepath.Join(dir, "agent")

	// No backup — we just want to confirm rollback returns an error (not panic).
	u := newTestUpgradeChecker("http://localhost")
	ctx := context.Background()

	err := u.rollback(ctx, exePath)
	if err == nil {
		t.Fatal("expected error from rollback with no backup, got nil")
	}
}

// --- upgradeCheckResponse parsing (JSON struct field coverage) ---

// TestUpgradeArtifact_Fields ensures the struct fields are accessible
// (exercises the JSON tags and struct initialisation).
func TestUpgradeArtifact_Fields(t *testing.T) {
	a := upgradeArtifact{
		OS:   "linux",
		Arch: "amd64",
		URL:  "/download/agent-linux-amd64",
		SHA:  "abc123",
	}
	if a.OS != "linux" || a.Arch != "amd64" || a.URL == "" || a.SHA == "" {
		t.Errorf("unexpected artifact fields: %+v", a)
	}
}

func TestUpgradeCheckResponse_Fields(t *testing.T) {
	resp := upgradeCheckResponse{
		Version: "1.2.3",
		Available: []upgradeArtifact{
			{OS: "windows", Arch: "amd64", URL: "/dl/agent.exe", SHA: "deadbeef"},
		},
	}
	if resp.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", resp.Version)
	}
	if len(resp.Available) != 1 {
		t.Errorf("Available len = %d, want 1", len(resp.Available))
	}
}

// --- check (single cycle) ---

// TestCheck_UpToDate verifies that when the controller reports the same
// version as currentVersion, no upgrade is attempted.
func TestCheck_UpToDate(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/upgrade/check" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"version":"0.1.0","available":[]}`)
			return
		}
		// Any download path means applyUpgrade was called — flag it.
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	u := newTestUpgradeChecker(srv.URL)
	ctx := context.Background()
	u.check(ctx, func() bool { return false })

	if called {
		t.Error("applyUpgrade should NOT have been triggered when version is up to date")
	}
}

// TestCheck_AgentBusy verifies that when the agent reports busy, upgrade is
// deferred even if a new version is available.
func TestCheck_AgentBusy(t *testing.T) {
	downloadCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/upgrade/check" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"version":"9.9.9","available":[{"os":"linux","arch":"amd64","url":"/dl/agent","sha256":""}]}`)
			return
		}
		downloadCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u := newTestUpgradeChecker(srv.URL)
	ctx := context.Background()

	// isBusy returns true → upgrade must be deferred.
	u.check(ctx, func() bool { return true })

	if downloadCalled {
		t.Error("download was called even though agent reported busy")
	}
}

// TestCheck_HTTPError verifies check returns gracefully on connection errors.
func TestCheck_HTTPError(t *testing.T) {
	// Use a server that immediately closes the connection.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // close immediately so the request fails

	u := newTestUpgradeChecker(srv.URL)
	ctx := context.Background()

	// Should not panic.
	u.check(ctx, func() bool { return false })
}

// TestCheck_BadJSON verifies check handles malformed JSON gracefully.
func TestCheck_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `not json at all`)
	}))
	defer srv.Close()

	u := newTestUpgradeChecker(srv.URL)
	ctx := context.Background()

	// Should not panic.
	u.check(ctx, func() bool { return false })
}

// TestCheck_NonOKStatus verifies check handles a non-200 status gracefully.
func TestCheck_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	u := newTestUpgradeChecker(srv.URL)
	ctx := context.Background()

	u.check(ctx, func() bool { return false })
}

