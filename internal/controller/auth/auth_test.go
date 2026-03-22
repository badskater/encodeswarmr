package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------------
// loginStubStore — controls GetUserByUsername and CreateSession responses
// ---------------------------------------------------------------------------

type loginStubStore struct {
	authStubStore
	user       *db.User
	userErr    error
	session    *db.Session
	sessionErr error
	deletedTok string
}

func (s *loginStubStore) GetUserByUsername(_ context.Context, _ string) (*db.User, error) {
	return s.user, s.userErr
}

func (s *loginStubStore) CreateSession(_ context.Context, p db.CreateSessionParams) (*db.Session, error) {
	if s.session != nil {
		return s.session, s.sessionErr
	}
	// Return a real session based on params when none is pre-configured.
	return &db.Session{
		Token:     p.Token,
		UserID:    p.UserID,
		ExpiresAt: p.ExpiresAt,
	}, s.sessionErr
}

func (s *loginStubStore) DeleteSession(_ context.Context, tok string) error {
	s.deletedTok = tok
	return nil
}

// ---------------------------------------------------------------------------
// TestLogin
// ---------------------------------------------------------------------------

func TestLogin(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate bcrypt hash: %v", err)
	}
	hashStr := string(hash)

	t.Run("user not found returns ErrInvalidCredentials", func(t *testing.T) {
		store := &loginStubStore{userErr: db.ErrNotFound}
		svc := &Service{store: store, cfg: &config.AuthConfig{SessionTTL: time.Hour}}

		_, err := svc.Login(context.Background(), "ghost", "pass")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("store error propagates", func(t *testing.T) {
		storeErr := errors.New("db down")
		store := &loginStubStore{userErr: storeErr}
		svc := &Service{store: store, cfg: &config.AuthConfig{SessionTTL: time.Hour}}

		_, err := svc.Login(context.Background(), "user", "pass")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, ErrInvalidCredentials) {
			t.Error("unexpected ErrInvalidCredentials for store error")
		}
	})

	t.Run("OIDC-only user (nil PasswordHash) returns ErrInvalidCredentials", func(t *testing.T) {
		store := &loginStubStore{
			user: &db.User{ID: "u1", Username: "alice", PasswordHash: nil},
		}
		svc := &Service{store: store, cfg: &config.AuthConfig{SessionTTL: time.Hour}}

		_, err := svc.Login(context.Background(), "alice", "any-password")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("expected ErrInvalidCredentials for OIDC-only user, got %v", err)
		}
	})

	t.Run("wrong password returns ErrInvalidCredentials", func(t *testing.T) {
		store := &loginStubStore{
			user: &db.User{ID: "u1", Username: "alice", PasswordHash: &hashStr},
		}
		svc := &Service{store: store, cfg: &config.AuthConfig{SessionTTL: time.Hour}}

		_, err := svc.Login(context.Background(), "alice", "wrong-password")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("expected ErrInvalidCredentials for wrong password, got %v", err)
		}
	})

	t.Run("correct password returns session", func(t *testing.T) {
		store := &loginStubStore{
			user: &db.User{ID: "u1", Username: "alice", PasswordHash: &hashStr},
		}
		svc := &Service{store: store, cfg: &config.AuthConfig{SessionTTL: time.Hour}}

		sess, err := svc.Login(context.Background(), "alice", "correct-password")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sess == nil {
			t.Fatal("expected session, got nil")
		}
		if sess.UserID != "u1" {
			t.Errorf("session.UserID = %q, want u1", sess.UserID)
		}
		if sess.Token == "" {
			t.Error("expected non-empty session token")
		}
	})
}

// ---------------------------------------------------------------------------
// TestLogout
// ---------------------------------------------------------------------------

func TestLogout(t *testing.T) {
	t.Run("calls DeleteSession with token", func(t *testing.T) {
		store := &loginStubStore{}
		svc := &Service{store: store, cfg: &config.AuthConfig{SessionTTL: time.Hour}}

		err := svc.Logout(context.Background(), "my-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if store.deletedTok != "my-token" {
			t.Errorf("DeleteSession called with %q, want %q", store.deletedTok, "my-token")
		}
	})
}

// ---------------------------------------------------------------------------
// TestHashPassword
// ---------------------------------------------------------------------------

func TestHashPassword(t *testing.T) {
	t.Run("returns non-empty hash", func(t *testing.T) {
		hash, err := HashPassword("my-secret-password")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hash == "" {
			t.Error("expected non-empty hash")
		}
	})

	t.Run("hash verifies correctly", func(t *testing.T) {
		password := "verify-me-123"
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
			t.Errorf("bcrypt.CompareHashAndPassword: %v", err)
		}
	})

	t.Run("two hashes of same password differ (salted)", func(t *testing.T) {
		a, _ := HashPassword("same-password")
		b, _ := HashPassword("same-password")
		if a == b {
			t.Error("expected different hashes for same password (bcrypt salting)")
		}
	})
}

// ---------------------------------------------------------------------------
// TestOIDCEnabled
// ---------------------------------------------------------------------------

func TestOIDCEnabled(t *testing.T) {
	t.Run("returns false when OIDC not configured", func(t *testing.T) {
		svc := &Service{}
		if svc.OIDCEnabled() {
			t.Error("OIDCEnabled() = true, want false")
		}
	})

	t.Run("returns true when OIDC is configured", func(t *testing.T) {
		svc := &Service{oidc: &oidcProvider{}}
		if !svc.OIDCEnabled() {
			t.Error("OIDCEnabled() = false, want true")
		}
	})
}

// ---------------------------------------------------------------------------
// TestNewService
// ---------------------------------------------------------------------------

func TestNewService(t *testing.T) {
	t.Run("OIDC disabled creates service without provider", func(t *testing.T) {
		store := &authStubStore{}
		cfg := &config.AuthConfig{
			SessionTTL: time.Hour,
			OIDC:       config.OIDCConfig{Enabled: false},
		}
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		svc, err := NewService(context.Background(), store, cfg, logger)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		if svc == nil {
			t.Fatal("expected non-nil service")
		}
		if svc.OIDCEnabled() {
			t.Error("expected OIDCEnabled() = false when OIDC disabled")
		}
	})

	t.Run("OIDC enabled with invalid provider URL returns error", func(t *testing.T) {
		store := &authStubStore{}
		cfg := &config.AuthConfig{
			OIDC: config.OIDCConfig{
				Enabled:     true,
				ProviderURL: "http://127.0.0.1:1", // unreachable
				ClientID:    "client",
			},
		}
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		_, err := NewService(context.Background(), store, cfg, logger)
		if err == nil {
			t.Fatal("expected error for invalid OIDC provider, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// TestGetSession
// ---------------------------------------------------------------------------

type getSessionStore struct {
	authStubStore
	session *db.Session
	sessErr error
	user    *db.User
	userErr error
}

func (s *getSessionStore) GetSessionByToken(_ context.Context, _ string) (*db.Session, error) {
	return s.session, s.sessErr
}
func (s *getSessionStore) GetUserByID(_ context.Context, _ string) (*db.User, error) {
	return s.user, s.userErr
}

func TestGetSession(t *testing.T) {
	t.Run("session not found returns ErrNotFound", func(t *testing.T) {
		store := &getSessionStore{sessErr: db.ErrNotFound}
		svc := &Service{store: store, cfg: &config.AuthConfig{SessionTTL: time.Hour}}

		_, _, err := svc.GetSession(context.Background(), "bad-token")
		if !errors.Is(err, db.ErrNotFound) {
			t.Errorf("expected db.ErrNotFound, got %v", err)
		}
	})

	t.Run("store error propagates", func(t *testing.T) {
		storeErr := errors.New("db error")
		store := &getSessionStore{sessErr: storeErr}
		svc := &Service{store: store, cfg: &config.AuthConfig{SessionTTL: time.Hour}}

		_, _, err := svc.GetSession(context.Background(), "any-token")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "get session") {
			t.Errorf("error %q doesn't mention 'get session'", err.Error())
		}
	})

	t.Run("valid session returns session and user", func(t *testing.T) {
		store := &getSessionStore{
			session: &db.Session{Token: "tok", UserID: "u1"},
			user:    &db.User{ID: "u1", Username: "alice", Role: "admin"},
		}
		svc := &Service{store: store, cfg: &config.AuthConfig{SessionTTL: time.Hour}}

		sess, user, err := svc.GetSession(context.Background(), "tok")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sess == nil {
			t.Fatal("expected session, got nil")
		}
		if user == nil {
			t.Fatal("expected user, got nil")
		}
		if user.Username != "alice" {
			t.Errorf("user.Username = %q, want alice", user.Username)
		}
	})

	t.Run("user lookup error propagates", func(t *testing.T) {
		store := &getSessionStore{
			session: &db.Session{Token: "tok", UserID: "u1"},
			userErr: errors.New("user not found"),
		}
		svc := &Service{store: store, cfg: &config.AuthConfig{SessionTTL: time.Hour}}

		_, _, err := svc.GetSession(context.Background(), "tok")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// TestRandomHex
// ---------------------------------------------------------------------------

func TestRandomHex(t *testing.T) {
	t.Run("returns hex string of expected length", func(t *testing.T) {
		hex, err := randomHex(16)
		if err != nil {
			t.Fatalf("randomHex: %v", err)
		}
		// 16 bytes → 32 hex chars
		if len(hex) != 32 {
			t.Errorf("len(hex) = %d, want 32", len(hex))
		}
	})

	t.Run("consecutive calls produce different values", func(t *testing.T) {
		a, err1 := randomHex(16)
		b, err2 := randomHex(16)
		if err1 != nil || err2 != nil {
			t.Fatalf("randomHex errors: %v, %v", err1, err2)
		}
		if a == b {
			t.Error("expected different random values")
		}
	})

	t.Run("32 bytes returns 64 char hex", func(t *testing.T) {
		hex, err := randomHex(32)
		if err != nil {
			t.Fatalf("randomHex: %v", err)
		}
		if len(hex) != 64 {
			t.Errorf("len(hex) = %d, want 64", len(hex))
		}
	})
}
