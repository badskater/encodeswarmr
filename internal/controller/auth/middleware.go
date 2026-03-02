package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/badskater/distributed-encoder/internal/db"
)

// SessionCookieName is the name of the HTTP session cookie.
const SessionCookieName = "session"

// Middleware validates the session cookie (or Authorization: Bearer header)
// and injects the authenticated claims into the request context.
// Unauthenticated requests receive a 401 response.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.extractToken(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, user, err := s.GetSession(r.Context(), token)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err != nil {
			s.logger.Error("session lookup failed", "err", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		claims := &Claims{
			UserID:   user.ID,
			Username: user.Username,
			Role:     user.Role,
		}
		next.ServeHTTP(w, r.WithContext(withClaims(r.Context(), claims)))
	})
}

// RequireRole wraps a handler and rejects requests where the caller's role
// is lower than minRole. Must be used after Middleware in the handler chain.
func RequireRole(minRole string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := FromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !hasRole(claims.Role, minRole) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractToken reads the session token from the cookie or Authorization header.
func (s *Service) extractToken(r *http.Request) string {
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		return cookie.Value
	}
	if hdr := r.Header.Get("Authorization"); strings.HasPrefix(hdr, "Bearer ") {
		return strings.TrimPrefix(hdr, "Bearer ")
	}
	return ""
}

// hasRole returns true when role satisfies the minimum requirement.
// Role precedence (ascending): viewer < operator < admin.
func hasRole(role, minRole string) bool {
	order := map[string]int{"viewer": 1, "operator": 2, "admin": 3}
	return order[role] >= order[minRole]
}
