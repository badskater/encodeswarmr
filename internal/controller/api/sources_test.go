package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// handleCreateSource
// ---------------------------------------------------------------------------

// TestHandleCreateSource_InvalidJSON verifies a 400 on malformed body.
func TestHandleCreateSource_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources", bytes.NewBufferString("not-json"))
	srv.handleCreateSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestHandleCreateSource_NonUNCPath verifies a 400 when the path is not UNC.
func TestHandleCreateSource_NonUNCPath(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources",
		bytes.NewBufferString(`{"path":"C:\\video\\file.mkv"}`))
	srv.handleCreateSource(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestHandleCreateSource_IdempotentExisting verifies that a 200 is returned
// (and no jobs are created) when the source already exists.
func TestHandleCreateSource_IdempotentExisting(t *testing.T) {
	store := &createSourceExistingStore{
		stubStore: &stubStore{},
		existing:  &db.Source{ID: "src-exists", Filename: "movie.mkv"},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources",
		bytes.NewBufferString(`{"path":"\\\\nas\\share\\movie.mkv"}`))
	srv.handleCreateSource(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if store.jobsCreated != 0 {
		t.Errorf("jobs created = %d, want 0 for idempotent source return", store.jobsCreated)
	}
}

// TestHandleCreateSource_NewSourceSchedulesAnalysis verifies that creating a
// new source automatically queues one "analysis" and one "hdr_detect" job.
func TestHandleCreateSource_NewSourceSchedulesAnalysis(t *testing.T) {
	store := &createSourceNewStore{
		stubStore: &stubStore{},
		created:   &db.Source{ID: "src-new", Filename: "film.mkv"},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources",
		bytes.NewBufferString(`{"path":"\\\\nas\\share\\film.mkv"}`))
	srv.handleCreateSource(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	// Exactly two jobs must be created: analysis + hdr_detect.
	if len(store.createdJobTypes) != 2 {
		t.Fatalf("jobs created = %d, want 2", len(store.createdJobTypes))
	}
	typeSet := make(map[string]bool, 2)
	for _, jt := range store.createdJobTypes {
		typeSet[jt] = true
	}
	for _, want := range []string{"analysis", "hdr_detect"} {
		if !typeSet[want] {
			t.Errorf("expected job type %q to be scheduled, got %v", want, store.createdJobTypes)
		}
	}
	// All auto-jobs must reference the newly created source.
	for _, sid := range store.createdSourceIDs {
		if sid != "src-new" {
			t.Errorf("auto-job source_id = %q, want %q", sid, "src-new")
		}
	}
}

// ---------------------------------------------------------------------------
// store stubs for handleCreateSource tests
// ---------------------------------------------------------------------------

type createSourceExistingStore struct {
	*stubStore
	existing    *db.Source
	jobsCreated int
}

func (s *createSourceExistingStore) GetSourceByUNCPath(context.Context, string) (*db.Source, error) {
	return s.existing, nil
}

func (s *createSourceExistingStore) CreateJob(context.Context, db.CreateJobParams) (*db.Job, error) {
	s.jobsCreated++
	return nil, nil
}

type createSourceNewStore struct {
	*stubStore
	created          *db.Source
	createdJobTypes  []string
	createdSourceIDs []string
}

func (s *createSourceNewStore) GetSourceByUNCPath(context.Context, string) (*db.Source, error) {
	return nil, db.ErrNotFound
}

func (s *createSourceNewStore) CreateSource(context.Context, db.CreateSourceParams) (*db.Source, error) {
	return s.created, nil
}

func (s *createSourceNewStore) CreateJob(_ context.Context, p db.CreateJobParams) (*db.Job, error) {
	s.createdJobTypes = append(s.createdJobTypes, p.JobType)
	s.createdSourceIDs = append(s.createdSourceIDs, p.SourceID)
	return &db.Job{ID: "auto-job", SourceID: p.SourceID, JobType: p.JobType}, nil
}
