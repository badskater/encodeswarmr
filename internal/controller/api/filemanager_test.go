package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// ---------------------------------------------------------------------------
// TestIsPathAllowed
// ---------------------------------------------------------------------------

func TestIsPathAllowed(t *testing.T) {
	allowed := []string{"/mnt/media", "/mnt/output"}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"exact match", "/mnt/media", true},
		{"subdirectory", "/mnt/media/movies", true},
		{"other allowed", "/mnt/output/done/file.mkv", true},
		{"not allowed", "/etc/passwd", false},
		{"prefix overlap but not subdir", "/mnt/media2", false},
		{"empty allowed", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathAllowed(tt.path, allowed)
			if got != tt.want {
				t.Errorf("isPathAllowed(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}

	t.Run("empty allowed list", func(t *testing.T) {
		if isPathAllowed("/mnt/media", nil) {
			t.Error("expected false for nil allowed list")
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleBrowseFiles
// ---------------------------------------------------------------------------

func TestHandleBrowseFiles(t *testing.T) {
	t.Run("missing path returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/browse", nil)
		srv.handleBrowseFiles(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("forbidden path returns 403", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{"/mnt/allowed"},
			},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/browse?path=/etc/secret", nil)
		srv.handleBrowseFiles(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})

	t.Run("nonexistent directory returns 404", func(t *testing.T) {
		tmpDir := t.TempDir()
		allowedDir := filepath.Join(tmpDir, "allowed")
		// Don't create the dir — it doesn't exist.

		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{tmpDir},
			},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/browse?path="+allowedDir, nil)
		srv.handleBrowseFiles(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("success lists directory contents", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create some test files.
		os.WriteFile(filepath.Join(tmpDir, "movie.mkv"), []byte("fake video"), 0o644)
		os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("text"), 0o644)
		os.Mkdir(filepath.Join(tmpDir, "subdir"), 0o755)

		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{tmpDir},
			},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/browse?path="+tmpDir, nil)
		srv.handleBrowseFiles(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data struct {
				Path    string      `json:"path"`
				Entries []FileEntry `json:"entries"`
			} `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data.Entries) != 3 {
			t.Errorf("len(entries) = %d, want 3", len(body.Data.Entries))
		}

		// Verify video detection.
		for _, e := range body.Data.Entries {
			if e.Name == "movie.mkv" {
				if !e.IsVideo {
					t.Error("expected movie.mkv to be detected as video")
				}
				if e.Ext != ".mkv" {
					t.Errorf("ext = %q, want .mkv", e.Ext)
				}
			}
			if e.Name == "subdir" {
				if !e.IsDir {
					t.Error("expected subdir to be a directory")
				}
			}
		}
	})

	t.Run("no allowed paths means 403", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{} // FileManager.AllowedPaths is nil.

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/browse?path=/tmp", nil)
		srv.handleBrowseFiles(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleFileInfo
// ---------------------------------------------------------------------------

func TestHandleFileInfo(t *testing.T) {
	t.Run("missing path returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/info", nil)
		srv.handleFileInfo(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("forbidden path returns 403", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{"/mnt/allowed"},
			},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/info?path=/etc/passwd", nil)
		srv.handleFileInfo(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})

	t.Run("nonexistent file returns 404", func(t *testing.T) {
		tmpDir := t.TempDir()

		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{tmpDir},
			},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/info?path="+filepath.Join(tmpDir, "nope.mkv"), nil)
		srv.handleFileInfo(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("success returns file metadata", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		os.WriteFile(testFile, []byte("hello"), 0o644)

		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{tmpDir},
			},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/info?path="+testFile, nil)
		srv.handleFileInfo(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data FileInfo `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data.Name != "test.txt" {
			t.Errorf("name = %q, want test.txt", body.Data.Name)
		}
		if body.Data.Size != 5 {
			t.Errorf("size = %d, want 5", body.Data.Size)
		}
		if body.Data.IsVideo {
			t.Error("expected txt file not to be video")
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleMoveFile
// ---------------------------------------------------------------------------

func TestHandleMoveFile(t *testing.T) {
	t.Run("invalid JSON returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/files/move", nil)
		srv.handleMoveFile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("missing source returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := `{"source":"","destination":"/mnt/dst"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/files/move", bytes.NewBufferString(body))
		srv.handleMoveFile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("forbidden source returns 403", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{"/mnt/allowed"},
			},
		}

		body := `{"source":"/etc/secret","destination":"/mnt/allowed/file"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/files/move", bytes.NewBufferString(body))
		srv.handleMoveFile(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})

	t.Run("forbidden destination returns 403", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{"/mnt/allowed"},
			},
		}

		body := `{"source":"/mnt/allowed/file","destination":"/etc/evil"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/files/move", bytes.NewBufferString(body))
		srv.handleMoveFile(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})

	t.Run("success moves file", func(t *testing.T) {
		tmpDir := t.TempDir()
		srcFile := filepath.Join(tmpDir, "src.txt")
		dstFile := filepath.Join(tmpDir, "dst.txt")
		os.WriteFile(srcFile, []byte("data"), 0o644)

		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{tmpDir},
			},
		}

		// Use json.Marshal for the request body to properly escape Windows backslashes.
		reqBody, _ := json.Marshal(map[string]string{
			"source":      srcFile,
			"destination": dstFile,
		})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/files/move", bytes.NewBuffer(reqBody))
		srv.handleMoveFile(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusOK, rr.Body.String())
		}
		// Verify source is gone and destination exists.
		if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
			t.Error("source file should no longer exist")
		}
		if _, err := os.Stat(dstFile); err != nil {
			t.Error("destination file should exist")
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleDeleteFile
// ---------------------------------------------------------------------------

func TestHandleDeleteFile(t *testing.T) {
	t.Run("missing path returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/?path=", nil)
		srv.handleDeleteFile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("forbidden path returns 403", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{"/mnt/allowed"},
			},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/?path=/etc/passwd", nil)
		srv.handleDeleteFile(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleDownloadFile
// ---------------------------------------------------------------------------

func TestHandleDownloadFile(t *testing.T) {
	t.Run("missing path returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/download", nil)
		srv.handleDownloadFile(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("forbidden path returns 403", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{"/mnt/allowed"},
			},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/download?path=/etc/passwd", nil)
		srv.handleDownloadFile(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})

	t.Run("nonexistent file returns 404", func(t *testing.T) {
		tmpDir := t.TempDir()

		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{tmpDir},
			},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/download?path="+filepath.Join(tmpDir, "missing.txt"), nil)
		srv.handleDownloadFile(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("success streams file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "download.txt")
		os.WriteFile(testFile, []byte("file content"), 0o644)

		srv := newTestServer(&stubStore{})
		srv.cfg = &config.Config{
			FileManager: config.FileManagerConfig{
				AllowedPaths: []string{tmpDir},
			},
		}

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files/download?path="+testFile, nil)
		srv.handleDownloadFile(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		ct := rr.Header().Get("Content-Type")
		if ct != "application/octet-stream" {
			t.Errorf("Content-Type = %q, want application/octet-stream", ct)
		}
		if !strings.Contains(rr.Body.String(), "file content") {
			t.Errorf("body = %q, want containing 'file content'", rr.Body.String())
		}
	})
}
