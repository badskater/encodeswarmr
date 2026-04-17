package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/encodeswarmr/internal/controller/engine"
	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// TestHandleListFlows
// ---------------------------------------------------------------------------

func TestHandleListFlows(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		store := &listFlowsStore{stubStore: &stubStore{}, flows: []*db.Flow{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/flows", nil)
		srv.handleListFlows(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body struct {
			Data []json.RawMessage `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 0 {
			t.Errorf("len(data) = %d, want 0", len(body.Data))
		}
	})

	t.Run("list with flows", func(t *testing.T) {
		store := &listFlowsStore{
			stubStore: &stubStore{},
			flows: []*db.Flow{
				{ID: "f1", Name: "pipeline-a", Graph: json.RawMessage(`{"nodes":[],"edges":[]}`)},
				{ID: "f2", Name: "pipeline-b", Graph: json.RawMessage(`{"nodes":[],"edges":[]}`)},
			},
		}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/flows", nil)
		srv.handleListFlows(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body struct {
			Data []json.RawMessage `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 2 {
			t.Errorf("len(data) = %d, want 2", len(body.Data))
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &listFlowsErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/flows", nil)
		srv.handleListFlows(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type listFlowsStore struct {
	*stubStore
	flows []*db.Flow
}

func (s *listFlowsStore) ListFlows(_ context.Context) ([]*db.Flow, error) {
	return s.flows, nil
}

type listFlowsErrStore struct {
	*stubStore
}

func (s *listFlowsErrStore) ListFlows(_ context.Context) ([]*db.Flow, error) {
	return nil, errors.New("db connection failed")
}

// ---------------------------------------------------------------------------
// TestHandleGetFlow
// ---------------------------------------------------------------------------

func TestHandleGetFlow(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		flow := &db.Flow{ID: "f1", Name: "my-pipeline", Graph: json.RawMessage(`{"nodes":[],"edges":[]}`)}
		store := &getFlowStore{stubStore: &stubStore{}, flow: flow}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/flows/f1", nil)
		req.SetPathValue("id", "f1")
		srv.handleGetFlow(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var body struct {
			Data db.Flow `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data.ID != "f1" {
			t.Errorf("data.ID = %q, want f1", body.Data.ID)
		}
		if body.Data.Name != "my-pipeline" {
			t.Errorf("data.Name = %q, want my-pipeline", body.Data.Name)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		store := &getFlowNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/flows/missing", nil)
		req.SetPathValue("id", "missing")
		srv.handleGetFlow(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &getFlowErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/flows/f1", nil)
		req.SetPathValue("id", "f1")
		srv.handleGetFlow(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type getFlowStore struct {
	*stubStore
	flow *db.Flow
}

func (s *getFlowStore) GetFlowByID(_ context.Context, _ string) (*db.Flow, error) {
	return s.flow, nil
}

type getFlowNotFoundStore struct {
	*stubStore
}

func (s *getFlowNotFoundStore) GetFlowByID(_ context.Context, _ string) (*db.Flow, error) {
	return nil, db.ErrNotFound
}

type getFlowErrStore struct {
	*stubStore
}

func (s *getFlowErrStore) GetFlowByID(_ context.Context, _ string) (*db.Flow, error) {
	return nil, errors.New("connection timeout")
}

// ---------------------------------------------------------------------------
// TestHandleCreateFlow
// ---------------------------------------------------------------------------

func TestHandleCreateFlow(t *testing.T) {
	validGraph := `{"nodes":[{"id":"n1","type":"input_source","data":{}}],"edges":[]}`

	t.Run("success returns 201", func(t *testing.T) {
		created := &db.Flow{
			ID:    "f-new",
			Name:  "encode-pipeline",
			Graph: json.RawMessage(validGraph),
		}
		store := &createFlowStore{stubStore: &stubStore{}, created: created}
		srv := newTestServer(store)

		body := bytes.NewBufferString(`{"name":"encode-pipeline","graph":` + validGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/flows", body)
		req.Header.Set("Content-Type", "application/json")
		srv.handleCreateFlow(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}

		var resp struct {
			Data db.Flow `json:"data"`
		}
		decodeJSON(t, rr, &resp)
		if resp.Data.ID != "f-new" {
			t.Errorf("data.ID = %q, want f-new", resp.Data.ID)
		}
	})

	t.Run("missing name returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := bytes.NewBufferString(`{"graph":` + validGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/flows", body)
		srv.handleCreateFlow(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("empty name returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := bytes.NewBufferString(`{"name":"","graph":` + validGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/flows", body)
		srv.handleCreateFlow(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("missing graph returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := bytes.NewBufferString(`{"name":"my-flow"}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/flows", body)
		srv.handleCreateFlow(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("invalid JSON body returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := bytes.NewBufferString(`{not valid json`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/flows", body)
		srv.handleCreateFlow(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &createFlowErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		body := bytes.NewBufferString(`{"name":"my-flow","graph":` + validGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/flows", body)
		srv.handleCreateFlow(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type createFlowStore struct {
	*stubStore
	created *db.Flow
}

func (s *createFlowStore) CreateFlow(_ context.Context, _ db.CreateFlowParams) (*db.Flow, error) {
	return s.created, nil
}

type createFlowErrStore struct {
	*stubStore
}

func (s *createFlowErrStore) CreateFlow(_ context.Context, _ db.CreateFlowParams) (*db.Flow, error) {
	return nil, errors.New("insert failed")
}

// ---------------------------------------------------------------------------
// TestHandleUpdateFlow
// ---------------------------------------------------------------------------

func TestHandleUpdateFlow(t *testing.T) {
	validGraph := `{"nodes":[{"id":"n1","type":"input_source","data":{}}],"edges":[]}`

	t.Run("success returns 200", func(t *testing.T) {
		updated := &db.Flow{
			ID:    "f1",
			Name:  "updated-pipeline",
			Graph: json.RawMessage(validGraph),
		}
		store := &updateFlowStore{stubStore: &stubStore{}, updated: updated}
		srv := newTestServer(store)

		body := bytes.NewBufferString(`{"name":"updated-pipeline","graph":` + validGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/flows/f1", body)
		req.SetPathValue("id", "f1")
		req.Header.Set("Content-Type", "application/json")
		srv.handleUpdateFlow(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var resp struct {
			Data db.Flow `json:"data"`
		}
		decodeJSON(t, rr, &resp)
		if resp.Data.Name != "updated-pipeline" {
			t.Errorf("data.Name = %q, want updated-pipeline", resp.Data.Name)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		store := &updateFlowNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		body := bytes.NewBufferString(`{"name":"updated","graph":` + validGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/flows/missing", body)
		req.SetPathValue("id", "missing")
		srv.handleUpdateFlow(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := bytes.NewBufferString(`{bad json`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/flows/f1", body)
		req.SetPathValue("id", "f1")
		srv.handleUpdateFlow(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing name returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		body := bytes.NewBufferString(`{"graph":` + validGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/flows/f1", body)
		req.SetPathValue("id", "f1")
		srv.handleUpdateFlow(rr, req)

		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &updateFlowErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		body := bytes.NewBufferString(`{"name":"my-flow","graph":` + validGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/flows/f1", body)
		req.SetPathValue("id", "f1")
		srv.handleUpdateFlow(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type updateFlowStore struct {
	*stubStore
	updated *db.Flow
}

func (s *updateFlowStore) UpdateFlow(_ context.Context, _ db.UpdateFlowParams) (*db.Flow, error) {
	return s.updated, nil
}

type updateFlowNotFoundStore struct {
	*stubStore
}

func (s *updateFlowNotFoundStore) UpdateFlow(_ context.Context, _ db.UpdateFlowParams) (*db.Flow, error) {
	return nil, db.ErrNotFound
}

type updateFlowErrStore struct {
	*stubStore
}

func (s *updateFlowErrStore) UpdateFlow(_ context.Context, _ db.UpdateFlowParams) (*db.Flow, error) {
	return nil, errors.New("update query failed")
}

// ---------------------------------------------------------------------------
// TestHandleDeleteFlow
// ---------------------------------------------------------------------------

func TestHandleDeleteFlow(t *testing.T) {
	t.Run("success returns 204", func(t *testing.T) {
		store := &deleteFlowStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/flows/f1", nil)
		req.SetPathValue("id", "f1")
		srv.handleDeleteFlow(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		store := &deleteFlowNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/flows/missing", nil)
		req.SetPathValue("id", "missing")
		srv.handleDeleteFlow(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &deleteFlowErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/flows/f1", nil)
		req.SetPathValue("id", "f1")
		srv.handleDeleteFlow(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type deleteFlowStore struct {
	*stubStore
}

func (s *deleteFlowStore) DeleteFlow(_ context.Context, _ string) error {
	return nil
}

type deleteFlowNotFoundStore struct {
	*stubStore
}

func (s *deleteFlowNotFoundStore) DeleteFlow(_ context.Context, _ string) error {
	return db.ErrNotFound
}

type deleteFlowErrStore struct {
	*stubStore
}

func (s *deleteFlowErrStore) DeleteFlow(_ context.Context, _ string) error {
	return errors.New("delete query failed")
}

// ---------------------------------------------------------------------------
// newTestServerWithFlowEngine creates a Server with a real FlowEngine wired
// in so that graph-validation paths are exercised in handler tests.
// ---------------------------------------------------------------------------

func newTestServerWithFlowEngine(store db.Store) *Server {
	srv := newTestServer(store)
	srv.flowEngine = engine.NewFlowEngine(store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return srv
}

// ---------------------------------------------------------------------------
// TestHandleCreateFlow_GraphValidation
// ---------------------------------------------------------------------------

func TestHandleCreateFlow_GraphValidation(t *testing.T) {
	// A structurally valid graph: one input node, no edges.
	validGraph := `{"nodes":[{"id":"n1","type":"input_source","data":{}}],"edges":[]}`

	// A cyclic graph: two nodes each pointing at the other.
	cyclicGraph := `{"nodes":[{"id":"n1","type":"input_source","data":{}},{"id":"n2","type":"encode","data":{}}],"edges":[{"id":"e1","source":"n1","target":"n2"},{"id":"e2","source":"n2","target":"n1"}]}`

	// An edge pointing to a node that does not exist.
	badEdgeGraph := `{"nodes":[{"id":"n1","type":"input_source","data":{}}],"edges":[{"id":"e1","source":"n1","target":"ghost"}]}`

	t.Run("valid flow is accepted (201)", func(t *testing.T) {
		created := &db.Flow{ID: "f-new", Name: "good-flow", Graph: json.RawMessage(validGraph)}
		store := &createFlowStore{stubStore: &stubStore{}, created: created}
		srv := newTestServerWithFlowEngine(store)

		body := bytes.NewBufferString(`{"name":"good-flow","graph":` + validGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/flows", body)
		srv.handleCreateFlow(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}
	})

	t.Run("cyclic graph is rejected (400)", func(t *testing.T) {
		srv := newTestServerWithFlowEngine(&stubStore{})

		body := bytes.NewBufferString(`{"name":"cyclic-flow","graph":` + cyclicGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/flows", body)
		srv.handleCreateFlow(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
		}

		var p problem
		if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
			t.Fatalf("decode problem body: %v", err)
		}
		if p.Detail == "" {
			t.Error("expected non-empty detail in problem response")
		}
	})

	t.Run("edge to non-existent node is rejected (400)", func(t *testing.T) {
		srv := newTestServerWithFlowEngine(&stubStore{})

		body := bytes.NewBufferString(`{"name":"bad-edge-flow","graph":` + badEdgeGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/flows", body)
		srv.handleCreateFlow(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
		}

		var p problem
		if err := json.NewDecoder(rr.Body).Decode(&p); err != nil {
			t.Fatalf("decode problem body: %v", err)
		}
		if p.Detail == "" {
			t.Error("expected non-empty detail in problem response")
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleUpdateFlow_GraphValidation
// ---------------------------------------------------------------------------

func TestHandleUpdateFlow_GraphValidation(t *testing.T) {
	// A cyclic graph.
	cyclicGraph := `{"nodes":[{"id":"n1","type":"input_source","data":{}},{"id":"n2","type":"encode","data":{}}],"edges":[{"id":"e1","source":"n1","target":"n2"},{"id":"e2","source":"n2","target":"n1"}]}`

	t.Run("update with invalid graph is rejected (400) and store not called", func(t *testing.T) {
		// updateCallStore records whether UpdateFlow was invoked.
		store := &updateFlowNotCalledStore{stubStore: &stubStore{}}
		srv := newTestServerWithFlowEngine(store)

		body := bytes.NewBufferString(`{"name":"still-valid","graph":` + cyclicGraph + `}`)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/flows/f1", body)
		req.SetPathValue("id", "f1")
		srv.handleUpdateFlow(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
		}
		if store.called {
			t.Error("UpdateFlow was called despite invalid graph — old flow may have been overwritten")
		}
	})
}

// updateFlowNotCalledStore lets tests assert that UpdateFlow was never invoked.
type updateFlowNotCalledStore struct {
	*stubStore
	called bool
}

func (s *updateFlowNotCalledStore) UpdateFlow(_ context.Context, _ db.UpdateFlowParams) (*db.Flow, error) {
	s.called = true
	return nil, errors.New("should not have been called")
}
