package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/badskater/distributed-encoder/internal/controller/config"
	"github.com/badskater/distributed-encoder/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials is returned when login fails due to bad username or password.
var ErrInvalidCredentials = errors.New("invalid credentials")

// Service handles local and OIDC authentication and session management.
type Service struct {
	store  db.Store
	cfg    *config.AuthConfig
	logger *slog.Logger
	oidc   *oidcProvider // nil if OIDC disabled
}

// NewService creates the auth service. Initialises the OIDC provider if enabled.
func NewService(ctx context.Context, store db.Store, cfg *config.AuthConfig, logger *slog.Logger) (*Service, error) {
	svc := &Service{store: store, cfg: cfg, logger: logger}
	if cfg.OIDC.Enabled {
		p, err := newOIDCProvider(ctx, &cfg.OIDC)
		if err != nil {
			return nil, fmt.Errorf("auth: init oidc provider: %w", err)
		}
		svc.oidc = p
	}
	return svc, nil
}

// Login authenticates a local user and creates a session on success.
func (s *Service) Login(ctx context.Context, username, password string) (*db.Session, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("auth: login: get user: %w", err)
	}
	if user.PasswordHash == nil {
		// OIDC-only user; cannot log in locally.
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return s.createSession(ctx, user.ID)
}

// GetSession validates a session token and returns the session and its owning user.
func (s *Service) GetSession(ctx context.Context, token string) (*db.Session, *db.User, error) {
	sess, err := s.store.GetSessionByToken(ctx, token)
	if errors.Is(err, db.ErrNotFound) {
		return nil, nil, db.ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("auth: get session: %w", err)
	}
	user, err := s.store.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("auth: get user for session: %w", err)
	}
	return sess, user, nil
}

// Logout deletes the session.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.store.DeleteSession(ctx, token)
}

// OIDCEnabled reports whether OIDC is configured and active.
func (s *Service) OIDCEnabled() bool { return s.oidc != nil }

// HashPassword bcrypt-hashes a plaintext password.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// createSession generates a random token and inserts a session row.
func (s *Service) createSession(ctx context.Context, userID string) (*db.Session, error) {
	token, err := randomHex(32)
	if err != nil {
		return nil, fmt.Errorf("auth: generate session token: %w", err)
	}
	return s.store.CreateSession(ctx, db.CreateSessionParams{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(s.cfg.SessionTTL),
	})
}

// randomHex returns n cryptographically random bytes encoded as a lowercase hex string.
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
