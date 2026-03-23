package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// TestHandleBatchImport
// ---------------------------------------------------------------------------

func TestHandleBatchImport(t *testing.T) {
	t.Run("missing path_pattern returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := `{"recursive": false}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/batch-import", bytes.NewBufferString(body))
		srv.handleBatchImport(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rr.Code)
		}
	})

	t.Run("invalid JSON body returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/batch-import", bytes.NewBufferString("not-json"))
		srv.handleBatchImport(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("valid request returns created sources", func(t *testing.T) {
		// Create a temp dir with a couple of .mkv files to glob.
		dir := t.TempDir()
		for _, name := range []string{"movie1.mkv", "movie2.mkv"} {
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("create test file: %v", err)
			}
			f.Close()
		}

		var created int
		store := &batchImportStore{
			stubStore: &stubStore{},
			createFn: func(p db.CreateSourceParams) (*db.Source, error) {
				created++
				return &db.Source{ID: "src-" + p.Filename, Filename: p.Filename, UNCPath: p.UNCPath}, nil
			},
		}
		srv := newTestServer(store)

		pattern := filepath.Join(dir, "*.mkv")
		body := `{"path_pattern":"` + escapeJSONString(pattern) + `"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/batch-import", bytes.NewBufferString(body))
		srv.handleBatchImport(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
		}

		var resp struct {
			Data struct {
				Imported int `json:"imported"`
				Results  []struct {
					SourceID string `json:"source_id"`
					Skipped  bool   `json:"skipped"`
				} `json:"results"`
			} `json:"data"`
		}
		decodeJSON(t, rr, &resp)

		if resp.Data.Imported != 2 {
			t.Errorf("imported = %d, want 2", resp.Data.Imported)
		}
		if created != 2 {
			t.Errorf("CreateSource called %d times, want 2", created)
		}
	})

	t.Run("idempotent: existing source is skipped", func(t *testing.T) {
		dir := t.TempDir()
		f, _ := os.Create(filepath.Join(dir, "dup.mkv"))
		f.Close()

		store := &batchImportExistingStore{
			stubStore: &stubStore{},
			existing:  &db.Source{ID: "src-existing", Filename: "dup.mkv"},
		}
		srv := newTestServer(store)

		pattern := filepath.Join(dir, "*.mkv")
		body := `{"path_pattern":"` + escapeJSONString(pattern) + `"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/batch-import", bytes.NewBufferString(body))
		srv.handleBatchImport(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var resp struct {
			Data struct {
				Results []struct {
					Skipped    bool   `json:"skipped"`
					SkipReason string `json:"skip_reason"`
				} `json:"results"`
			} `json:"data"`
		}
		decodeJSON(t, rr, &resp)
		if len(resp.Data.Results) != 1 {
			t.Fatalf("len(results) = %d, want 1", len(resp.Data.Results))
		}
		if !resp.Data.Results[0].Skipped {
			t.Error("expected result to be skipped")
		}
	})

	t.Run("store error on CreateSource records error in result", func(t *testing.T) {
		dir := t.TempDir()
		f, _ := os.Create(filepath.Join(dir, "fail.mkv"))
		f.Close()

		store := &batchImportErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		pattern := filepath.Join(dir, "*.mkv")
		body := `{"path_pattern":"` + escapeJSONString(pattern) + `"}`
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/batch-import", bytes.NewBufferString(body))
		srv.handleBatchImport(rr, req)

		// The handler returns 200 even when individual file creation fails.
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var resp struct {
			Data struct {
				Results []struct {
					Error string `json:"error"`
				} `json:"results"`
			} `json:"data"`
		}
		decodeJSON(t, rr, &resp)
		if len(resp.Data.Results) != 1 {
			t.Fatalf("len(results) = %d, want 1", len(resp.Data.Results))
		}
		if resp.Data.Results[0].Error == "" {
			t.Error("expected error in result, got empty string")
		}
	})
}

// ---------------------------------------------------------------------------
// Store stubs for batch import tests
// ---------------------------------------------------------------------------

type batchImportStore struct {
	*stubStore
	createFn func(db.CreateSourceParams) (*db.Source, error)
}

func (s *batchImportStore) CreateSource(_ context.Context, p db.CreateSourceParams) (*db.Source, error) {
	return s.createFn(p)
}

// GetSourceByUNCPath returns not-found to simulate a new source.
func (s *batchImportStore) GetSourceByUNCPath(_ context.Context, _ string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

type batchImportExistingStore struct {
	*stubStore
	existing *db.Source
}

func (s *batchImportExistingStore) GetSourceByUNCPath(_ context.Context, _ string) (*db.Source, error) {
	return s.existing, nil
}

type batchImportErrStore struct{ *stubStore }

func (s *batchImportErrStore) GetSourceByUNCPath(_ context.Context, _ string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

func (s *batchImportErrStore) CreateSource(_ context.Context, _ db.CreateSourceParams) (*db.Source, error) {
	return nil, errors.New("db failure")
}

// ---------------------------------------------------------------------------
// escapeJSONString replaces backslashes with double-backslash for JSON string
// literals. Used only in tests.
// ---------------------------------------------------------------------------

func escapeJSONString(s string) string {
	out := make([]byte, 0, len(s)+4)
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			out = append(out, '\\', '\\')
		} else {
			out = append(out, s[i])
		}
	}
	return string(out)
}
