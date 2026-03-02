package auth

import (
	"context"
	"errors"
	"fmt"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/badskater/distributed-encoder/internal/controller/config"
	"github.com/badskater/distributed-encoder/internal/db"
)

type oidcProvider struct {
	verifier *gooidc.IDTokenVerifier
	oauth2   oauth2.Config
	cfg      *config.OIDCConfig
}

func newOIDCProvider(ctx context.Context, cfg *config.OIDCConfig) (*oidcProvider, error) {
	p, err := gooidc.NewProvider(ctx, cfg.ProviderURL)
	if err != nil {
		return nil, fmt.Errorf("oidc: discover provider %q: %w", cfg.ProviderURL, err)
	}
	oa2cfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     p.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
	}
	verifier := p.Verifier(&gooidc.Config{ClientID: cfg.ClientID})
	return &oidcProvider{
		verifier: verifier,
		oauth2:   oa2cfg,
		cfg:      cfg,
	}, nil
}

// OIDCRedirectURL returns the authorization URL to redirect the browser to.
func (s *Service) OIDCRedirectURL(state string) (string, error) {
	if s.oidc == nil {
		return "", fmt.Errorf("auth: OIDC not configured")
	}
	return s.oidc.oauth2.AuthCodeURL(state), nil
}

// OIDCCallback exchanges the authorization code for tokens, verifies the ID
// token, and creates or retrieves the user account before issuing a session.
func (s *Service) OIDCCallback(ctx context.Context, code string) (*db.Session, error) {
	if s.oidc == nil {
		return nil, fmt.Errorf("auth: OIDC not configured")
	}
	token, err := s.oidc.oauth2.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("auth: oidc exchange: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("auth: oidc: missing id_token in response")
	}
	idToken, err := s.oidc.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("auth: oidc: id_token verify: %w", err)
	}
	var claims struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("auth: oidc: extract claims: %w", err)
	}

	// Look up existing user by OIDC subject.
	user, err := s.store.GetUserByOIDCSub(ctx, claims.Sub)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return nil, fmt.Errorf("auth: oidc: get user: %w", err)
	}
	if errors.Is(err, db.ErrNotFound) {
		if !s.oidc.cfg.AutoProvision {
			return nil, fmt.Errorf("auth: oidc: user not found and auto-provision is disabled")
		}
		username := claims.Name
		if username == "" {
			username = claims.Email
		}
		oidcSub := claims.Sub
		user, err = s.store.CreateUser(ctx, db.CreateUserParams{
			Username: username,
			Email:    claims.Email,
			Role:     "viewer",
			OIDCSub:  &oidcSub,
		})
		if err != nil {
			return nil, fmt.Errorf("auth: oidc: create user: %w", err)
		}
		s.logger.Info("auto-provisioned OIDC user", "username", user.Username, "sub", claims.Sub)
	}
	return s.createSession(ctx, user.ID)
}
