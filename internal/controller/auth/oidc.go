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
// When RoleMappings are configured the user's groups claim is inspected and the
// highest-privilege matching role is applied on every login.
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

	// Decode all claims into a raw map so we can extract both the standard
	// fields and the configurable groups claim.
	var rawClaims map[string]any
	if err := idToken.Claims(&rawClaims); err != nil {
		return nil, fmt.Errorf("auth: oidc: extract claims: %w", err)
	}
	sub, _ := rawClaims["sub"].(string)
	email, _ := rawClaims["email"].(string)
	preferredUsername, _ := rawClaims["preferred_username"].(string)

	// Resolve the role from the groups claim when mappings are configured.
	role := s.resolveOIDCRole(rawClaims)

	// Look up existing user by OIDC subject.
	user, err := s.store.GetUserByOIDCSub(ctx, sub)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return nil, fmt.Errorf("auth: oidc: get user: %w", err)
	}
	if errors.Is(err, db.ErrNotFound) {
		if !s.oidc.cfg.AutoProvision {
			return nil, fmt.Errorf("auth: oidc: user not found and auto-provision is disabled")
		}
		username := preferredUsername
		if username == "" {
			username = email
		}
		oidcSub := sub
		user, err = s.store.CreateUser(ctx, db.CreateUserParams{
			Username: username,
			Email:    email,
			Role:     role,
			OIDCSub:  &oidcSub,
		})
		if err != nil {
			return nil, fmt.Errorf("auth: oidc: create user: %w", err)
		}
		s.logger.Info("auto-provisioned OIDC user", "username", user.Username, "sub", sub, "role", role)
	} else if user.Role != role && len(s.oidc.cfg.RoleMappings) > 0 {
		// Sync the role on subsequent logins when mappings are active.
		if err := s.store.UpdateUserRole(ctx, user.ID, role); err != nil {
			s.logger.Warn("oidc: failed to sync user role", "user_id", user.ID, "role", role, "err", err)
		} else {
			user.Role = role
		}
	}
	return s.createSession(ctx, user.ID)
}

// resolveOIDCRole extracts the groups claim from raw token claims and returns
// the highest-privilege role that has a mapping entry.  Falls back to
// DefaultRole (default: "viewer") when no mapping matches or when no mappings
// are configured.
func (s *Service) resolveOIDCRole(rawClaims map[string]any) string {
	cfg := s.oidc.cfg

	// If no role mappings are defined, return the configured default.
	if len(cfg.RoleMappings) == 0 {
		if cfg.DefaultRole != "" {
			return cfg.DefaultRole
		}
		return "viewer"
	}

	groupsClaim := cfg.GroupsClaim
	if groupsClaim == "" {
		groupsClaim = "groups"
	}

	// The groups claim may be []string or []any.
	var groups []string
	switch v := rawClaims[groupsClaim].(type) {
	case []string:
		groups = v
	case []any:
		for _, g := range v {
			if gs, ok := g.(string); ok {
				groups = append(groups, gs)
			}
		}
	}

	roleOrder := map[string]int{"viewer": 1, "operator": 2, "admin": 3}
	best := ""
	bestRank := 0
	for _, g := range groups {
		if mapped, ok := cfg.RoleMappings[g]; ok {
			if rank := roleOrder[mapped]; rank > bestRank {
				bestRank = rank
				best = mapped
			}
		}
	}
	if best != "" {
		return best
	}

	if cfg.DefaultRole != "" {
		return cfg.DefaultRole
	}
	return "viewer"
}
