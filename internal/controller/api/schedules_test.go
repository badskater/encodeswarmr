package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// validCronExpr is a standard 5-field cron expression accepted by the
// scheduler package.  "Every minute" is the simplest always-valid expression.
const validCronExpr = "* * * * *"

// ---------------------------------------------------------------------------
// handleListSchedules
// ---------------------------------------------------------------------------

func TestHandleListSchedules_Success(t *testing.T) {
	store := &listSchedulesStore{
		stubStore: &stubStore{},
		schedules: []*db.Schedule{
			{
				ID: "sc1", Name: "Nightly", CronExpr: "0 2 * * *",
				JobTemplate: json.RawMessage(`{"source_id":"s1","job_type":"encode"}`),
				Enabled:     true, CreatedAt: time.Now(),
			},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil)
	srv.handleListSchedules(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleListSchedules_StoreError(t *testing.T) {
	store := &listSchedulesErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/schedules", nil)
	srv.handleListSchedules(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleGetSchedule
// ---------------------------------------------------------------------------

func TestHandleGetSchedule_Success(t *testing.T) {
	store := &getScheduleStore{
		stubStore: &stubStore{},
		sc: &db.Schedule{
			ID: "sc1", Name: "Nightly", CronExpr: "0 2 * * *",
			JobTemplate: json.RawMessage(`{"source_id":"s1","job_type":"encode"}`),
			Enabled:     true, CreatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/schedules/sc1", nil)
	req.SetPathValue("id", "sc1")
	srv.handleGetSchedule(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeJSON(t, rr, &body)
	if body.Data.ID != "sc1" {
		t.Errorf("data.id = %q, want %q", body.Data.ID, "sc1")
	}
}

func TestHandleGetSchedule_NotFound(t *testing.T) {
	store := &getScheduleNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/schedules/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleGetSchedule(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleGetSchedule_StoreError(t *testing.T) {
	store := &getScheduleErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/schedules/sc1", nil)
	req.SetPathValue("id", "sc1")
	srv.handleGetSchedule(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleCreateSchedule
// ---------------------------------------------------------------------------

func TestHandleCreateSchedule_Success(t *testing.T) {
	store := &createScheduleStore{
		stubStore: &stubStore{},
		sc: &db.Schedule{
			ID: "sc-new", Name: "Nightly", CronExpr: validCronExpr,
			JobTemplate: json.RawMessage(`{"source_id":"s1","job_type":"encode"}`),
			Enabled:     true, CreatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	body := `{"name":"Nightly","cron_expr":"` + validCronExpr + `","job_template":{"source_id":"s1","job_type":"encode"}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedules", bytes.NewBufferString(body))
	srv.handleCreateSchedule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestHandleCreateSchedule_CreatedDisabled(t *testing.T) {
	store := &createScheduleParamsStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"Nightly","cron_expr":"` + validCronExpr + `","job_template":{"source_id":"s1","job_type":"encode"},"enabled":false}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedules", bytes.NewBufferString(body))
	srv.handleCreateSchedule(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
	// When created as disabled, next_run_at must be nil.
	if store.gotParams.NextRunAt != nil {
		t.Error("next_run_at should be nil for a disabled schedule")
	}
}

func TestHandleCreateSchedule_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedules", bytes.NewBufferString("bad"))
	srv.handleCreateSchedule(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateSchedule_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"cron_expr":"` + validCronExpr + `","job_template":{"source_id":"s1"}}`},
		{"missing cron_expr", `{"name":"N","job_template":{"source_id":"s1"}}`},
		{"missing job_template", `{"name":"N","cron_expr":"` + validCronExpr + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(&stubStore{})
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/schedules", bytes.NewBufferString(tc.body))
			srv.handleCreateSchedule(rr, req)
			if rr.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s: status = %d, want %d", tc.name, rr.Code, http.StatusUnprocessableEntity)
			}
		})
	}
}

func TestHandleCreateSchedule_InvalidCronExpr(t *testing.T) {
	srv := newTestServer(&stubStore{})

	body := `{"name":"N","cron_expr":"not a cron","job_template":{"source_id":"s1"}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedules", bytes.NewBufferString(body))
	srv.handleCreateSchedule(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestHandleCreateSchedule_StoreError(t *testing.T) {
	store := &createScheduleErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"N","cron_expr":"` + validCronExpr + `","job_template":{"source_id":"s1"}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedules", bytes.NewBufferString(body))
	srv.handleCreateSchedule(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleUpdateSchedule
// ---------------------------------------------------------------------------

func TestHandleUpdateSchedule_Success(t *testing.T) {
	store := &updateScheduleStore{
		stubStore: &stubStore{},
		sc: &db.Schedule{
			ID: "sc1", Name: "Updated", CronExpr: validCronExpr,
			JobTemplate: json.RawMessage(`{"source_id":"s1","job_type":"encode"}`),
			Enabled:     true, CreatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	body := `{"name":"Updated","cron_expr":"` + validCronExpr + `","job_template":{"source_id":"s1","job_type":"encode"}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/schedules/sc1", bytes.NewBufferString(body))
	req.SetPathValue("id", "sc1")
	srv.handleUpdateSchedule(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleUpdateSchedule_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/schedules/sc1", bytes.NewBufferString("bad"))
	req.SetPathValue("id", "sc1")
	srv.handleUpdateSchedule(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateSchedule_InvalidCronExpr(t *testing.T) {
	srv := newTestServer(&stubStore{})

	body := `{"name":"N","cron_expr":"bad cron","job_template":{"source_id":"s1"}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/schedules/sc1", bytes.NewBufferString(body))
	req.SetPathValue("id", "sc1")
	srv.handleUpdateSchedule(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestHandleUpdateSchedule_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"cron_expr":"` + validCronExpr + `","job_template":{"source_id":"s1"}}`},
		{"missing cron_expr", `{"name":"N","job_template":{"source_id":"s1"}}`},
		{"missing job_template", `{"name":"N","cron_expr":"` + validCronExpr + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(&stubStore{})
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/schedules/sc1", bytes.NewBufferString(tc.body))
			req.SetPathValue("id", "sc1")
			srv.handleUpdateSchedule(rr, req)
			if rr.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s: status = %d, want %d", tc.name, rr.Code, http.StatusUnprocessableEntity)
			}
		})
	}
}

func TestHandleUpdateSchedule_NotFound(t *testing.T) {
	store := &updateScheduleNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"N","cron_expr":"` + validCronExpr + `","job_template":{"source_id":"s1"}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/schedules/missing", bytes.NewBufferString(body))
	req.SetPathValue("id", "missing")
	srv.handleUpdateSchedule(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateSchedule_StoreError(t *testing.T) {
	store := &updateScheduleErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"N","cron_expr":"` + validCronExpr + `","job_template":{"source_id":"s1"}}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/schedules/sc1", bytes.NewBufferString(body))
	req.SetPathValue("id", "sc1")
	srv.handleUpdateSchedule(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleUpdateSchedule_DisabledNextRunNil(t *testing.T) {
	store := &updateScheduleParamsStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"N","cron_expr":"` + validCronExpr + `","job_template":{"source_id":"s1"},"enabled":false}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/schedules/sc1", bytes.NewBufferString(body))
	req.SetPathValue("id", "sc1")
	srv.handleUpdateSchedule(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if store.gotParams.NextRunAt != nil {
		t.Error("next_run_at should be nil when schedule is disabled")
	}
}

// ---------------------------------------------------------------------------
// handleDeleteSchedule
// ---------------------------------------------------------------------------

func TestHandleDeleteSchedule_Success(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/schedules/sc1", nil)
	req.SetPathValue("id", "sc1")
	srv.handleDeleteSchedule(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHandleDeleteSchedule_NotFound(t *testing.T) {
	store := &deleteScheduleNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/schedules/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleDeleteSchedule(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleDeleteSchedule_StoreError(t *testing.T) {
	store := &deleteScheduleErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/schedules/sc1", nil)
	req.SetPathValue("id", "sc1")
	srv.handleDeleteSchedule(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// store stubs for schedules tests
// ---------------------------------------------------------------------------

type listSchedulesStore struct {
	*stubStore
	schedules []*db.Schedule
}

func (s *listSchedulesStore) ListSchedules(_ context.Context) ([]*db.Schedule, error) {
	return s.schedules, nil
}

type listSchedulesErrStore struct{ *stubStore }

func (s *listSchedulesErrStore) ListSchedules(_ context.Context) ([]*db.Schedule, error) {
	return nil, errors.New("db failure")
}

type getScheduleStore struct {
	*stubStore
	sc *db.Schedule
}

func (s *getScheduleStore) GetScheduleByID(_ context.Context, _ string) (*db.Schedule, error) {
	return s.sc, nil
}

type getScheduleNotFoundStore struct{ *stubStore }

func (s *getScheduleNotFoundStore) GetScheduleByID(_ context.Context, _ string) (*db.Schedule, error) {
	return nil, db.ErrNotFound
}

type getScheduleErrStore struct{ *stubStore }

func (s *getScheduleErrStore) GetScheduleByID(_ context.Context, _ string) (*db.Schedule, error) {
	return nil, errors.New("db failure")
}

type createScheduleStore struct {
	*stubStore
	sc *db.Schedule
}

func (s *createScheduleStore) CreateSchedule(_ context.Context, _ db.CreateScheduleParams) (*db.Schedule, error) {
	return s.sc, nil
}

type createScheduleParamsStore struct {
	*stubStore
	gotParams db.CreateScheduleParams
}

func (s *createScheduleParamsStore) CreateSchedule(_ context.Context, p db.CreateScheduleParams) (*db.Schedule, error) {
	s.gotParams = p
	return &db.Schedule{
		ID: "sc-p", Name: p.Name, CronExpr: p.CronExpr,
		JobTemplate: p.JobTemplate, Enabled: p.Enabled, CreatedAt: time.Now(),
	}, nil
}

type createScheduleErrStore struct{ *stubStore }

func (s *createScheduleErrStore) CreateSchedule(_ context.Context, _ db.CreateScheduleParams) (*db.Schedule, error) {
	return nil, errors.New("db failure")
}

type updateScheduleStore struct {
	*stubStore
	sc *db.Schedule
}

func (s *updateScheduleStore) UpdateSchedule(_ context.Context, _ db.UpdateScheduleParams) (*db.Schedule, error) {
	return s.sc, nil
}

type updateScheduleNotFoundStore struct{ *stubStore }

func (s *updateScheduleNotFoundStore) UpdateSchedule(_ context.Context, _ db.UpdateScheduleParams) (*db.Schedule, error) {
	return nil, db.ErrNotFound
}

type updateScheduleErrStore struct{ *stubStore }

func (s *updateScheduleErrStore) UpdateSchedule(_ context.Context, _ db.UpdateScheduleParams) (*db.Schedule, error) {
	return nil, errors.New("db failure")
}

type updateScheduleParamsStore struct {
	*stubStore
	gotParams db.UpdateScheduleParams
}

func (s *updateScheduleParamsStore) UpdateSchedule(_ context.Context, p db.UpdateScheduleParams) (*db.Schedule, error) {
	s.gotParams = p
	return &db.Schedule{
		ID: p.ID, Name: p.Name, CronExpr: p.CronExpr,
		JobTemplate: p.JobTemplate, Enabled: p.Enabled, CreatedAt: time.Now(),
	}, nil
}

type deleteScheduleNotFoundStore struct{ *stubStore }

func (s *deleteScheduleNotFoundStore) DeleteSchedule(_ context.Context, _ string) error {
	return db.ErrNotFound
}

type deleteScheduleErrStore struct{ *stubStore }

func (s *deleteScheduleErrStore) DeleteSchedule(_ context.Context, _ string) error {
	return errors.New("db failure")
}
