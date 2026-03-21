package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/db"
)

// generateTestCSRPEM returns a PEM-encoded certificate signing request for use
// in unit tests.  A fresh EC key pair is generated each call.
func generateTestCSRPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	tmpl := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "test-agent"},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		t.Fatalf("create CSR: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}))
}

// zeroConfig returns a minimal *config.Config with empty values (TLS disabled).
func zeroConfig() *config.Config {
	return &config.Config{}
}

// ---------------------------------------------------------------------------
// TestToEnrollmentTokenResponse
// ---------------------------------------------------------------------------

func TestToEnrollmentTokenResponse(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	t.Run("used_at nil", func(t *testing.T) {
		tok := &db.EnrollmentToken{
			ID:        "tok-1",
			Token:     "secret",
			ExpiresAt: now.Add(24 * time.Hour),
			CreatedAt: now,
		}
		resp := toEnrollmentTokenResponse(tok)
		if resp.ID != "tok-1" {
			t.Errorf("ID = %q, want tok-1", resp.ID)
		}
		if resp.UsedAt != nil {
			t.Error("expected UsedAt to be nil")
		}
	})

	t.Run("used_at set", func(t *testing.T) {
		usedAt := now.Add(-time.Hour)
		tok := &db.EnrollmentToken{
			ID:        "tok-2",
			Token:     "secret",
			UsedAt:    &usedAt,
			ExpiresAt: now.Add(24 * time.Hour),
			CreatedAt: now,
		}
		resp := toEnrollmentTokenResponse(tok)
		if resp.UsedAt == nil {
			t.Error("expected UsedAt to be set")
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleCreateEnrollmentToken
// ---------------------------------------------------------------------------

func TestHandleCreateEnrollmentToken(t *testing.T) {
	t.Run("invalid JSON returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-tokens",
			bytes.NewBufferString(`{not json`))
		srv.handleCreateEnrollmentToken(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid expires_in returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-tokens",
			bytes.NewBufferString(`{"expires_in":"notaduration"}`))
		srv.handleCreateEnrollmentToken(rr, req)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &createTokenErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-tokens",
			bytes.NewBufferString(`{}`))
		srv.handleCreateEnrollmentToken(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success with default expiry", func(t *testing.T) {
		now := time.Now()
		store := &createTokenSuccessStore{
			stubStore: &stubStore{},
			tok: &db.EnrollmentToken{
				ID:        "t1",
				Token:     "abc123",
				ExpiresAt: now.Add(24 * time.Hour),
				CreatedAt: now,
			},
		}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-tokens",
			bytes.NewBufferString(`{}`))
		srv.handleCreateEnrollmentToken(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}
		var body struct {
			Data struct {
				ID    string `json:"id"`
				Token string `json:"token"`
			} `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data.ID != "t1" {
			t.Errorf("data.id = %q, want t1", body.Data.ID)
		}
		if body.Data.Token != "abc123" {
			t.Errorf("data.token = %q, want abc123", body.Data.Token)
		}
	})

	t.Run("success with custom expiry", func(t *testing.T) {
		now := time.Now()
		store := &createTokenSuccessStore{
			stubStore: &stubStore{},
			tok: &db.EnrollmentToken{
				ID:        "t2",
				Token:     "xyz",
				ExpiresAt: now.Add(2 * time.Hour),
				CreatedAt: now,
			},
		}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-tokens",
			bytes.NewBufferString(`{"expires_in":"2h"}`))
		srv.handleCreateEnrollmentToken(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
		}
	})
}

type createTokenErrStore struct{ *stubStore }

func (s *createTokenErrStore) CreateEnrollmentToken(_ context.Context, _ db.CreateEnrollmentTokenParams) (*db.EnrollmentToken, error) {
	return nil, errTestDB
}

type createTokenSuccessStore struct {
	*stubStore
	tok *db.EnrollmentToken
}

func (s *createTokenSuccessStore) CreateEnrollmentToken(_ context.Context, _ db.CreateEnrollmentTokenParams) (*db.EnrollmentToken, error) {
	return s.tok, nil
}

// ---------------------------------------------------------------------------
// TestHandleListEnrollmentTokens
// ---------------------------------------------------------------------------

func TestHandleListEnrollmentTokens(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		now := time.Now()
		store := &listTokensStore{
			stubStore: &stubStore{},
			tokens: []*db.EnrollmentToken{
				{ID: "t1", Token: "secret1", ExpiresAt: now.Add(time.Hour), CreatedAt: now},
				{ID: "t2", Token: "secret2", ExpiresAt: now.Add(time.Hour), CreatedAt: now},
			},
		}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent-tokens", nil)
		srv.handleListEnrollmentTokens(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 2 {
			t.Errorf("len(data) = %d, want 2", len(body.Data))
		}
		// Token values must not be exposed in list.
		for _, item := range body.Data {
			if _, hasToken := item["token"]; hasToken {
				t.Error("token field must not appear in list response")
			}
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &listTokensErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent-tokens", nil)
		srv.handleListEnrollmentTokens(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})
}

type listTokensStore struct {
	*stubStore
	tokens []*db.EnrollmentToken
}

func (s *listTokensStore) ListEnrollmentTokens(_ context.Context) ([]*db.EnrollmentToken, error) {
	return s.tokens, nil
}

type listTokensErrStore struct{ *stubStore }

func (s *listTokensErrStore) ListEnrollmentTokens(_ context.Context) ([]*db.EnrollmentToken, error) {
	return nil, errTestDB
}

// ---------------------------------------------------------------------------
// TestHandleDeleteEnrollmentToken
// ---------------------------------------------------------------------------

func TestHandleDeleteEnrollmentToken(t *testing.T) {
	t.Run("not found returns 404", func(t *testing.T) {
		store := &deleteTokenNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/agent-tokens/missing", nil)
		req.SetPathValue("id", "missing")
		srv.handleDeleteEnrollmentToken(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
		}
	})

	t.Run("store error returns 500", func(t *testing.T) {
		store := &deleteTokenErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/agent-tokens/t1", nil)
		req.SetPathValue("id", "t1")
		srv.handleDeleteEnrollmentToken(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("success returns 204", func(t *testing.T) {
		store := &deleteTokenSuccessStore{stubStore: &stubStore{}}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/agent-tokens/t1", nil)
		req.SetPathValue("id", "t1")
		srv.handleDeleteEnrollmentToken(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
		}
	})
}

type deleteTokenNotFoundStore struct{ *stubStore }

func (s *deleteTokenNotFoundStore) DeleteEnrollmentToken(_ context.Context, _ string) error {
	return db.ErrNotFound
}

type deleteTokenErrStore struct{ *stubStore }

func (s *deleteTokenErrStore) DeleteEnrollmentToken(_ context.Context, _ string) error {
	return errTestDB
}

type deleteTokenSuccessStore struct{ *stubStore }

func (s *deleteTokenSuccessStore) DeleteEnrollmentToken(_ context.Context, _ string) error {
	return nil
}

// ---------------------------------------------------------------------------
// TestHandleAgentEnroll
// ---------------------------------------------------------------------------

func TestHandleAgentEnroll(t *testing.T) {
	t.Run("invalid JSON returns 400", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/enroll",
			bytes.NewBufferString(`{not json`))
		srv.handleAgentEnroll(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing token returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/enroll",
			bytes.NewBufferString(`{"csr_pem":"somecsr"}`))
		srv.handleAgentEnroll(rr, req)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("missing csr_pem returns 422", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/enroll",
			bytes.NewBufferString(`{"token":"tok"}`))
		srv.handleAgentEnroll(rr, req)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnprocessableEntity)
		}
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		store := &enrollTokenNotFoundStore{stubStore: &stubStore{}}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/enroll",
			bytes.NewBufferString(`{"token":"badtok","csr_pem":"somecsr"}`))
		srv.handleAgentEnroll(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("token store error returns 500", func(t *testing.T) {
		store := &enrollTokenGetErrStore{stubStore: &stubStore{}}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/enroll",
			bytes.NewBufferString(`{"token":"tok","csr_pem":"somecsr"}`))
		srv.handleAgentEnroll(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
		}
	})

	t.Run("invalid PEM CSR returns 400", func(t *testing.T) {
		now := time.Now()
		store := &enrollTokenFoundStore{
			stubStore: &stubStore{},
			tok: &db.EnrollmentToken{
				ID: "t1", Token: "validtok",
				ExpiresAt: now.Add(time.Hour), CreatedAt: now,
			},
		}
		srv := newTestServer(store)
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/enroll",
			bytes.NewBufferString(`{"token":"validtok","csr_pem":"notvalidpem"}`))
		srv.handleAgentEnroll(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("TLS not configured returns 503 after valid token + valid PEM CSR", func(t *testing.T) {
		now := time.Now()
		store := &enrollTokenFoundStore{
			stubStore: &stubStore{},
			tok: &db.EnrollmentToken{
				ID: "t1", Token: "validtok",
				ExpiresAt: now.Add(time.Hour), CreatedAt: now,
			},
		}
		srv := newTestServer(store)
		// Provide a zero-value config so cfg.TLS.CertFile == "" (TLS not configured).
		srv.cfg = zeroConfig()

		csrPEM := generateTestCSRPEM(t)
		bodyBytes, err := json.Marshal(map[string]string{
			"token":   "validtok",
			"csr_pem": csrPEM,
		})
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/enroll",
			bytes.NewBuffer(bodyBytes))
		srv.handleAgentEnroll(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
		}
	})
}

type enrollTokenNotFoundStore struct{ *stubStore }

func (s *enrollTokenNotFoundStore) GetEnrollmentToken(_ context.Context, _ string) (*db.EnrollmentToken, error) {
	return nil, db.ErrNotFound
}

type enrollTokenGetErrStore struct{ *stubStore }

func (s *enrollTokenGetErrStore) GetEnrollmentToken(_ context.Context, _ string) (*db.EnrollmentToken, error) {
	return nil, errTestDB
}

type enrollTokenFoundStore struct {
	*stubStore
	tok *db.EnrollmentToken
}

func (s *enrollTokenFoundStore) GetEnrollmentToken(_ context.Context, _ string) (*db.EnrollmentToken, error) {
	return s.tok, nil
}

