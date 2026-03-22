package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// handleListWebhooks
// ---------------------------------------------------------------------------

func TestHandleListWebhooks_Success(t *testing.T) {
	store := &listWebhooksStore{
		stubStore: &stubStore{},
		webhooks: []*db.Webhook{
			{ID: "wh1", Name: "Discord alerts", Provider: "discord", URL: "https://discord.com/api/webhooks/123", Events: []string{"job.completed"}},
			{ID: "wh2", Name: "Teams", Provider: "teams", URL: "https://outlook.office.com/webhook/abc", Events: []string{"job.failed"}},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks", nil)
	srv.handleListWebhooks(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleListWebhooks_StoreError(t *testing.T) {
	store := &listWebhooksErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks", nil)
	srv.handleListWebhooks(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleGetWebhook
// ---------------------------------------------------------------------------

func TestHandleGetWebhook_Success(t *testing.T) {
	store := &getWebhookStore{
		stubStore: &stubStore{},
		wh: &db.Webhook{
			ID: "wh1", Name: "Discord", Provider: "discord",
			URL: "https://discord.com/api/webhooks/123", Events: []string{"job.completed"},
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/wh1", nil)
	req.SetPathValue("id", "wh1")
	srv.handleGetWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleGetWebhook_NotFound(t *testing.T) {
	store := &getWebhookNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleGetWebhook(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleGetWebhook_StoreError(t *testing.T) {
	store := &getWebhookErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/wh1", nil)
	req.SetPathValue("id", "wh1")
	srv.handleGetWebhook(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleCreateWebhook
// ---------------------------------------------------------------------------

func TestHandleCreateWebhook_Success(t *testing.T) {
	store := &createWebhookStore{
		stubStore: &stubStore{},
		wh: &db.Webhook{
			ID: "wh-new", Name: "Slack", Provider: "slack",
			URL: "https://hooks.slack.com/services/abc", Events: []string{"job.completed"},
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	body := `{"name":"Slack","provider":"slack","url":"https://hooks.slack.com/services/abc","events":["job.completed"]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewBufferString(body))
	srv.handleCreateWebhook(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestHandleCreateWebhook_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewBufferString("not-json"))
	srv.handleCreateWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateWebhook_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"provider":"discord","url":"https://discord.com/abc","events":["job.completed"]}`},
		{"missing provider", `{"name":"W","url":"https://discord.com/abc","events":["job.completed"]}`},
		{"missing url", `{"name":"W","provider":"discord","events":["job.completed"]}`},
		{"missing events", `{"name":"W","provider":"discord","url":"https://discord.com/abc"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(&stubStore{})
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewBufferString(tc.body))
			srv.handleCreateWebhook(rr, req)
			if rr.Code != http.StatusUnprocessableEntity {
				t.Fatalf("%s: status = %d, want %d", tc.name, rr.Code, http.StatusUnprocessableEntity)
			}
		})
	}
}

func TestHandleCreateWebhook_InvalidProvider(t *testing.T) {
	srv := newTestServer(&stubStore{})

	body := `{"name":"W","provider":"unknown","url":"https://example.com","events":["job.completed"]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewBufferString(body))
	srv.handleCreateWebhook(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
	}
}

func TestHandleCreateWebhook_ValidProviders(t *testing.T) {
	for _, provider := range []string{"discord", "teams", "slack"} {
		t.Run("provider="+provider, func(t *testing.T) {
			store := &createWebhookStore{
				stubStore: &stubStore{},
				wh: &db.Webhook{
					ID: "wh-" + provider, Name: "W", Provider: provider,
					URL: "https://example.com", Events: []string{"job.completed"},
					CreatedAt: time.Now(), UpdatedAt: time.Now(),
				},
			}
			srv := newTestServer(store)

			body := `{"name":"W","provider":"` + provider + `","url":"https://example.com","events":["job.completed"]}`
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewBufferString(body))
			srv.handleCreateWebhook(rr, req)

			if rr.Code != http.StatusCreated {
				t.Errorf("provider=%q: status = %d, want %d", provider, rr.Code, http.StatusCreated)
			}
		})
	}
}

func TestHandleCreateWebhook_WithSecret(t *testing.T) {
	store := &createWebhookSecretStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"W","provider":"slack","url":"https://example.com","events":["job.completed"],"secret":"my-secret"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewBufferString(body))
	srv.handleCreateWebhook(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
	if store.gotParams.Secret == nil || *store.gotParams.Secret != "my-secret" {
		t.Errorf("secret not passed to store correctly")
	}
}

func TestHandleCreateWebhook_StoreError(t *testing.T) {
	store := &createWebhookErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"W","provider":"discord","url":"https://discord.com/abc","events":["job.completed"]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks", bytes.NewBufferString(body))
	srv.handleCreateWebhook(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleUpdateWebhook
// ---------------------------------------------------------------------------

func TestHandleUpdateWebhook_Success(t *testing.T) {
	store := &updateWebhookStore{
		stubStore: &stubStore{},
		wh: &db.Webhook{
			ID: "wh1", Name: "Updated", Provider: "discord",
			URL: "https://discord.com/abc", Events: []string{"job.completed"},
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	body := `{"name":"Updated","url":"https://discord.com/abc","events":["job.completed"],"enabled":true}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/webhooks/wh1", bytes.NewBufferString(body))
	req.SetPathValue("id", "wh1")
	srv.handleUpdateWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleUpdateWebhook_InvalidJSON(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/webhooks/wh1", bytes.NewBufferString("bad"))
	req.SetPathValue("id", "wh1")
	srv.handleUpdateWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateWebhook_NotFound(t *testing.T) {
	store := &updateWebhookNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"W","url":"https://example.com","events":["job.completed"]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/webhooks/missing", bytes.NewBufferString(body))
	req.SetPathValue("id", "missing")
	srv.handleUpdateWebhook(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateWebhook_StoreError(t *testing.T) {
	store := &updateWebhookErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	body := `{"name":"W","url":"https://example.com","events":["job.completed"]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/webhooks/wh1", bytes.NewBufferString(body))
	req.SetPathValue("id", "wh1")
	srv.handleUpdateWebhook(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleDeleteWebhook
// ---------------------------------------------------------------------------

func TestHandleDeleteWebhook_Success(t *testing.T) {
	srv := newTestServer(&stubStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/webhooks/wh1", nil)
	req.SetPathValue("id", "wh1")
	srv.handleDeleteWebhook(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHandleDeleteWebhook_NotFound(t *testing.T) {
	store := &deleteWebhookNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/webhooks/missing", nil)
	req.SetPathValue("id", "missing")
	srv.handleDeleteWebhook(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleDeleteWebhook_StoreError(t *testing.T) {
	store := &deleteWebhookErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/webhooks/wh1", nil)
	req.SetPathValue("id", "wh1")
	srv.handleDeleteWebhook(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// handleTestWebhook — uses a local HTTP test server to exercise the delivery
// ---------------------------------------------------------------------------

func TestHandleTestWebhook_NotFound(t *testing.T) {
	store := &testWebhookNotFoundStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/missing/test", nil)
	req.SetPathValue("id", "missing")
	srv.handleTestWebhook(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleTestWebhook_StoreError(t *testing.T) {
	store := &testWebhookGetErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/wh1/test", nil)
	req.SetPathValue("id", "wh1")
	srv.handleTestWebhook(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleTestWebhook_DeliverySuccess(t *testing.T) {
	// Stand up a local HTTP server that returns 200.
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer targetSrv.Close()

	store := &testWebhookDeliverStore{
		stubStore: &stubStore{},
		wh: &db.Webhook{
			ID: "wh1", Name: "Test", Provider: "discord",
			URL: targetSrv.URL, Events: []string{"test"},
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/wh1/test", nil)
	req.SetPathValue("id", "wh1")
	srv.handleTestWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleTestWebhook_DeliveryNon2xx(t *testing.T) {
	// Target server returns 500.
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer targetSrv.Close()

	store := &testWebhookDeliverStore{
		stubStore: &stubStore{},
		wh: &db.Webhook{
			ID: "wh2", Name: "Bad", Provider: "slack",
			URL: targetSrv.URL, Events: []string{"test"},
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/wh2/test", nil)
	req.SetPathValue("id", "wh2")
	srv.handleTestWebhook(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadGateway)
	}
}

func TestHandleTestWebhook_DeliveryConnFailed(t *testing.T) {
	// Use an unreachable URL to force a connection error.
	store := &testWebhookDeliverStore{
		stubStore: &stubStore{},
		wh: &db.Webhook{
			ID: "wh3", Name: "Bad conn", Provider: "teams",
			URL: "http://127.0.0.1:0/no-server", Events: []string{"test"},
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/wh3/test", nil)
	req.SetPathValue("id", "wh3")
	srv.handleTestWebhook(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadGateway)
	}
}

// ---------------------------------------------------------------------------
// handleListWebhookDeliveries
// ---------------------------------------------------------------------------

func TestHandleListWebhookDeliveries_Success(t *testing.T) {
	store := &listDeliveriesStore{
		stubStore: &stubStore{},
		deliveries: []*db.WebhookDelivery{
			{ID: 1, WebhookID: "wh1", Event: "job.completed", Success: true},
		},
	}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/wh1/deliveries", nil)
	req.SetPathValue("id", "wh1")
	srv.handleListWebhookDeliveries(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleListWebhookDeliveries_StoreError(t *testing.T) {
	store := &listDeliveriesErrStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/wh1/deliveries", nil)
	req.SetPathValue("id", "wh1")
	srv.handleListWebhookDeliveries(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandleListWebhookDeliveries_PaginationParams(t *testing.T) {
	store := &listDeliveriesPaginationStore{stubStore: &stubStore{}}
	srv := newTestServer(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/wh1/deliveries?limit=10&offset=20", nil)
	req.SetPathValue("id", "wh1")
	srv.handleListWebhookDeliveries(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if store.gotLimit != 10 {
		t.Errorf("limit = %d, want 10", store.gotLimit)
	}
	if store.gotOffset != 20 {
		t.Errorf("offset = %d, want 20", store.gotOffset)
	}
}

// ---------------------------------------------------------------------------
// store stubs for webhooks tests
// ---------------------------------------------------------------------------

type listWebhooksStore struct {
	*stubStore
	webhooks []*db.Webhook
}

func (s *listWebhooksStore) ListWebhooks(_ context.Context) ([]*db.Webhook, error) {
	return s.webhooks, nil
}

type listWebhooksErrStore struct{ *stubStore }

func (s *listWebhooksErrStore) ListWebhooks(_ context.Context) ([]*db.Webhook, error) {
	return nil, errors.New("db failure")
}

type getWebhookStore struct {
	*stubStore
	wh *db.Webhook
}

func (s *getWebhookStore) GetWebhookByID(_ context.Context, _ string) (*db.Webhook, error) {
	return s.wh, nil
}

type getWebhookNotFoundStore struct{ *stubStore }

func (s *getWebhookNotFoundStore) GetWebhookByID(_ context.Context, _ string) (*db.Webhook, error) {
	return nil, db.ErrNotFound
}

type getWebhookErrStore struct{ *stubStore }

func (s *getWebhookErrStore) GetWebhookByID(_ context.Context, _ string) (*db.Webhook, error) {
	return nil, errors.New("db failure")
}

type createWebhookStore struct {
	*stubStore
	wh *db.Webhook
}

func (s *createWebhookStore) CreateWebhook(_ context.Context, _ db.CreateWebhookParams) (*db.Webhook, error) {
	return s.wh, nil
}

type createWebhookSecretStore struct {
	*stubStore
	gotParams db.CreateWebhookParams
}

func (s *createWebhookSecretStore) CreateWebhook(_ context.Context, p db.CreateWebhookParams) (*db.Webhook, error) {
	s.gotParams = p
	return &db.Webhook{
		ID: "wh-s", Name: p.Name, Provider: p.Provider, URL: p.URL,
		Events: p.Events, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil
}

type createWebhookErrStore struct{ *stubStore }

func (s *createWebhookErrStore) CreateWebhook(_ context.Context, _ db.CreateWebhookParams) (*db.Webhook, error) {
	return nil, errors.New("db failure")
}

type updateWebhookStore struct {
	*stubStore
	wh *db.Webhook
}

func (s *updateWebhookStore) UpdateWebhook(_ context.Context, _ db.UpdateWebhookParams) error {
	return nil
}

func (s *updateWebhookStore) GetWebhookByID(_ context.Context, _ string) (*db.Webhook, error) {
	return s.wh, nil
}

type updateWebhookNotFoundStore struct{ *stubStore }

func (s *updateWebhookNotFoundStore) UpdateWebhook(_ context.Context, _ db.UpdateWebhookParams) error {
	return db.ErrNotFound
}

type updateWebhookErrStore struct{ *stubStore }

func (s *updateWebhookErrStore) UpdateWebhook(_ context.Context, _ db.UpdateWebhookParams) error {
	return errors.New("db failure")
}

type deleteWebhookNotFoundStore struct{ *stubStore }

func (s *deleteWebhookNotFoundStore) DeleteWebhook(_ context.Context, _ string) error {
	return db.ErrNotFound
}

type deleteWebhookErrStore struct{ *stubStore }

func (s *deleteWebhookErrStore) DeleteWebhook(_ context.Context, _ string) error {
	return errors.New("db failure")
}

type testWebhookNotFoundStore struct{ *stubStore }

func (s *testWebhookNotFoundStore) GetWebhookByID(_ context.Context, _ string) (*db.Webhook, error) {
	return nil, db.ErrNotFound
}

type testWebhookGetErrStore struct{ *stubStore }

func (s *testWebhookGetErrStore) GetWebhookByID(_ context.Context, _ string) (*db.Webhook, error) {
	return nil, errors.New("db failure")
}

type testWebhookDeliverStore struct {
	*stubStore
	wh *db.Webhook
}

func (s *testWebhookDeliverStore) GetWebhookByID(_ context.Context, _ string) (*db.Webhook, error) {
	return s.wh, nil
}

type listDeliveriesStore struct {
	*stubStore
	deliveries []*db.WebhookDelivery
}

func (s *listDeliveriesStore) ListWebhookDeliveries(_ context.Context, _ string, _, _ int) ([]*db.WebhookDelivery, error) {
	return s.deliveries, nil
}

type listDeliveriesErrStore struct{ *stubStore }

func (s *listDeliveriesErrStore) ListWebhookDeliveries(_ context.Context, _ string, _, _ int) ([]*db.WebhookDelivery, error) {
	return nil, errors.New("db failure")
}

type listDeliveriesPaginationStore struct {
	*stubStore
	gotLimit  int
	gotOffset int
}

func (s *listDeliveriesPaginationStore) ListWebhookDeliveries(_ context.Context, _ string, limit, offset int) ([]*db.WebhookDelivery, error) {
	s.gotLimit = limit
	s.gotOffset = offset
	return []*db.WebhookDelivery{}, nil
}
