package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	_ "crypto/sha256" // register the SHA-256 hash for crypto.SHA256.New()
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/badskater/distributed-encoder/internal/controller/config"
	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// mockOIDCServer — minimal OIDC provider backed by net/http/httptest
// ---------------------------------------------------------------------------

// mockOIDCServer serves the three endpoints the go-oidc library requires:
//   - GET  /.well-known/openid-configuration
//   - GET  /jwks
//   - POST /token  (behaviour controlled by tokenFunc)
type mockOIDCServer struct {
	srv        *httptest.Server
	privateKey *rsa.PrivateKey
	// tokenFunc is called for every POST /token request.
	tokenFunc func(w http.ResponseWriter, r *http.Request)
}

// newMockOIDCServer starts a test HTTP server, registers cleanup, and returns
// the mock. Callers must set tokenFunc before initiating any token exchange.
func newMockOIDCServer(t *testing.T) *mockOIDCServer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	m := &mockOIDCServer{privateKey: key}

	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		doc := map[string]any{
			"issuer":                                m.srv.URL,
			"authorization_endpoint":                m.srv.URL + "/authorize",
			"token_endpoint":                        m.srv.URL + "/token",
			"jwks_uri":                              m.srv.URL + "/jwks",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		pub := &key.PublicKey
		n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
		jwks := map[string]any{
			"keys": []map[string]any{
				{"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "testkey", "n": n, "e": e},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if m.tokenFunc != nil {
			m.tokenFunc(w, r)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})

	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

// buildIDToken constructs a minimal RS256-signed JWT suitable for the mock
// OIDC server. Pass a past time as expiresAt to produce an expired token.
func (m *mockOIDCServer) buildIDToken(t *testing.T, clientID, sub, email, username string, audience []string, expiresAt time.Time) string {
	t.Helper()

	headerJSON, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": "testkey"})
	if err != nil {
		t.Fatalf("marshal JWT header: %v", err)
	}
	claimsJSON, err := json.Marshal(map[string]any{
		"iss":                m.srv.URL,
		"sub":                sub,
		"aud":                audience,
		"exp":                expiresAt.Unix(),
		"iat":                time.Now().Add(-5 * time.Second).Unix(),
		"email":              email,
		"preferred_username": username,
	})
	if err != nil {
		t.Fatalf("marshal JWT claims: %v", err)
	}

	sigInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)

	hasher := crypto.SHA256.New()
	hasher.Write([]byte(sigInput))
	digest := hasher.Sum(nil)
	sig, err := rsa.SignPKCS1v15(rand.Reader, m.privateKey, crypto.SHA256, digest)
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}

	return sigInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// tokenResponse builds the JSON body for a successful /token endpoint response.
func tokenResponse(t *testing.T, idToken string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"access_token": "at-placeholder",
		"token_type":   "bearer",
		"expires_in":   3600,
		"id_token":     idToken,
	})
	if err != nil {
		t.Fatalf("marshal token response: %v", err)
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// oidcStubStore — extends authStubStore with per-test OIDC return control
// ---------------------------------------------------------------------------

type oidcStubStore struct {
	authStubStore
	oidcUser       *db.User
	oidcUserErr    error
	createdUser    *db.User
	createUserErr  error
	createdSession *db.Session
	createSessErr  error
}

func (s *oidcStubStore) GetUserByOIDCSub(_ context.Context, _ string) (*db.User, error) {
	return s.oidcUser, s.oidcUserErr
}

func (s *oidcStubStore) CreateUser(_ context.Context, _ db.CreateUserParams) (*db.User, error) {
	return s.createdUser, s.createUserErr
}

func (s *oidcStubStore) CreateSession(_ context.Context, _ db.CreateSessionParams) (*db.Session, error) {
	return s.createdSession, s.createSessErr
}

// ---------------------------------------------------------------------------
// newOIDCService — builds a real auth.Service wired to the mock OIDC server
// ---------------------------------------------------------------------------

func newOIDCService(ctx context.Context, t *testing.T, m *mockOIDCServer, store db.Store, autoProvision bool) *Service {
	t.Helper()
	cfg := &config.AuthConfig{
		SessionTTL: time.Hour,
		OIDC: config.OIDCConfig{
			Enabled:       true,
			ProviderURL:   m.srv.URL,
			ClientID:      "test-client",
			ClientSecret:  "test-secret",
			RedirectURL:   "http://localhost/callback",
			AutoProvision: autoProvision,
		},
	}
	// Use a discard logger so the service never panics on s.logger.Info calls.
	discardLog := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := NewService(ctx, store, cfg, discardLog)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

// ---------------------------------------------------------------------------
// TestOIDCCallback — table-driven end-to-end OIDC callback tests
// ---------------------------------------------------------------------------

func TestOIDCCallback(t *testing.T) {
	const clientID = "test-client"

	tests := []struct {
		name          string
		autoProvision bool
		tokenFunc     func(m *mockOIDCServer) func(w http.ResponseWriter, r *http.Request)
		storeSetup    func() *oidcStubStore
		wantErr       string // substring that must appear in the error
		wantSession   bool   // assert non-nil session is returned
	}{
		{
			name:          "valid token exchange returns session",
			autoProvision: false,
			tokenFunc: func(m *mockOIDCServer) func(w http.ResponseWriter, r *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					tok := m.buildIDToken(t, clientID, "sub-alice", "alice@example.com", "alice",
						[]string{clientID}, time.Now().Add(time.Hour))
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, tokenResponse(t, tok))
				}
			},
			storeSetup: func() *oidcStubStore {
				return &oidcStubStore{
					oidcUser: &db.User{ID: "u-alice", Username: "alice", Role: "viewer"},
					createdSession: &db.Session{
						Token:     "sess-abc",
						UserID:    "u-alice",
						ExpiresAt: time.Now().Add(time.Hour),
					},
				}
			},
			wantSession: true,
		},
		{
			name:          "expired token is rejected",
			autoProvision: false,
			tokenFunc: func(m *mockOIDCServer) func(w http.ResponseWriter, r *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					tok := m.buildIDToken(t, clientID, "sub-exp", "exp@example.com", "expired",
						[]string{clientID}, time.Now().Add(-time.Hour)) // past expiry
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, tokenResponse(t, tok))
				}
			},
			storeSetup: func() *oidcStubStore { return &oidcStubStore{} },
			wantErr:    "id_token verify",
		},
		{
			name:          "invalid audience is rejected",
			autoProvision: false,
			tokenFunc: func(m *mockOIDCServer) func(w http.ResponseWriter, r *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					tok := m.buildIDToken(t, clientID, "sub-aud", "aud@example.com", "badaud",
						[]string{"wrong-client"}, // audience does not match ClientID
						time.Now().Add(time.Hour))
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, tokenResponse(t, tok))
				}
			},
			storeSetup: func() *oidcStubStore { return &oidcStubStore{} },
			wantErr:    "id_token verify",
		},
		{
			name:          "missing id_token field returns error",
			autoProvision: false,
			tokenFunc: func(m *mockOIDCServer) func(w http.ResponseWriter, r *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					// Valid OAuth2 token response but no id_token.
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"access_token": "at-only",
						"token_type":   "bearer",
						"expires_in":   3600,
					})
				}
			},
			storeSetup: func() *oidcStubStore { return &oidcStubStore{} },
			wantErr:    "missing id_token",
		},
		{
			name:          "user not found with auto-provision creates user and returns session",
			autoProvision: true,
			tokenFunc: func(m *mockOIDCServer) func(w http.ResponseWriter, r *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					tok := m.buildIDToken(t, clientID, "sub-new", "new@example.com", "newuser",
						[]string{clientID}, time.Now().Add(time.Hour))
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, tokenResponse(t, tok))
				}
			},
			storeSetup: func() *oidcStubStore {
				sub := "sub-new"
				return &oidcStubStore{
					oidcUserErr: db.ErrNotFound,
					createdUser: &db.User{ID: "u-new", Username: "newuser", Role: "viewer", OIDCSub: &sub},
					createdSession: &db.Session{
						Token:     "sess-new",
						UserID:    "u-new",
						ExpiresAt: time.Now().Add(time.Hour),
					},
				}
			},
			wantSession: true,
		},
		{
			name:          "user not found with auto-provision disabled returns error",
			autoProvision: false,
			tokenFunc: func(m *mockOIDCServer) func(w http.ResponseWriter, r *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					tok := m.buildIDToken(t, clientID, "sub-noprov", "noprov@example.com", "noprov",
						[]string{clientID}, time.Now().Add(time.Hour))
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, tokenResponse(t, tok))
				}
			},
			storeSetup:  func() *oidcStubStore { return &oidcStubStore{oidcUserErr: db.ErrNotFound} },
			wantErr:     "auto-provision is disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockOIDCServer(t)
			m.tokenFunc = tt.tokenFunc(m)
			store := tt.storeSetup()

			ctx := context.Background()
			svc := newOIDCService(ctx, t, m, store, tt.autoProvision)

			sess, err := svc.OIDCCallback(ctx, "auth-code")

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (sess=%+v)", tt.wantErr, sess)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain expected substring %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantSession && sess == nil {
				t.Fatal("expected non-nil session, got nil")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestOIDCNotConfigured
// ---------------------------------------------------------------------------

func TestOIDCNotConfigured(t *testing.T) {
	svc := newTestService(&authStubStore{})

	t.Run("OIDCCallback returns error", func(t *testing.T) {
		_, err := svc.OIDCCallback(context.Background(), "any-code")
		if err == nil {
			t.Fatal("expected error when OIDC is not configured")
		}
		if !strings.Contains(err.Error(), "OIDC not configured") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("OIDCRedirectURL returns error", func(t *testing.T) {
		_, err := svc.OIDCRedirectURL("state-xyz")
		if err == nil {
			t.Fatal("expected error when OIDC is not configured")
		}
	})
}

// ---------------------------------------------------------------------------
// TestOIDCRedirectURL — valid URL is returned when OIDC is configured
// ---------------------------------------------------------------------------

func TestOIDCRedirectURL(t *testing.T) {
	m := newMockOIDCServer(t)
	svc := newOIDCService(context.Background(), t, m, &oidcStubStore{}, false)

	url, err := svc.OIDCRedirectURL("my-state-value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(url, "my-state-value") {
		t.Errorf("redirect URL %q does not embed the state parameter", url)
	}
}

// ---------------------------------------------------------------------------
// TestNewOIDCProvider — verifies provider discovery and field population
// ---------------------------------------------------------------------------

func TestNewOIDCProvider(t *testing.T) {
	m := newMockOIDCServer(t)

	cfg := &config.OIDCConfig{
		Enabled:      true,
		ProviderURL:  m.srv.URL,
		ClientID:     "client-xyz",
		ClientSecret: "secret-xyz",
		RedirectURL:  "http://localhost/cb",
	}

	p, err := newOIDCProvider(context.Background(), cfg)
	if err != nil {
		t.Fatalf("newOIDCProvider: %v", err)
	}

	if p.verifier == nil {
		t.Error("verifier is nil after provider initialisation")
	}
	if p.oauth2.ClientID != "client-xyz" {
		t.Errorf("ClientID = %q, want client-xyz", p.oauth2.ClientID)
	}
	if p.oauth2.RedirectURL != "http://localhost/cb" {
		t.Errorf("RedirectURL = %q, want http://localhost/cb", p.oauth2.RedirectURL)
	}

	// Scopes must include openid.
	foundOpenID := false
	for _, s := range p.oauth2.Scopes {
		if s == gooidc.ScopeOpenID {
			foundOpenID = true
			break
		}
	}
	if !foundOpenID {
		t.Errorf("scopes %v do not include %q", p.oauth2.Scopes, gooidc.ScopeOpenID)
	}
}
