package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/distributed-encoder/internal/controller/auth"
	"github.com/badskater/distributed-encoder/internal/db"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "username and password are required")
		return
	}
	sess, err := s.auth.Login(r.Context(), req.Username, req.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeProblem(w, r, http.StatusUnauthorized, "Unauthorized", "invalid username or password")
		return
	}
	if err != nil {
		s.logger.Error("login error", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	setSessionCookie(w, sess)
	writeJSON(w, r, http.StatusOK, map[string]any{"expires_at": sess.ExpiresAt})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		// No session cookie; treat as already logged out.
		writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if err := s.auth.Logout(r.Context(), cookie.Value); err != nil {
		s.logger.Error("logout error", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	clearSessionCookie(w)
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleOIDCRedirect(w http.ResponseWriter, r *http.Request) {
	if !s.auth.OIDCEnabled() {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "OIDC is not enabled")
		return
	}
	state, err := genOIDCState()
	if err != nil {
		s.logger.Error("generate OIDC state", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	// Store state in a short-lived cookie for CSRF protection.
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	url, err := s.auth.OIDCRedirectURL(state)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if !s.auth.OIDCEnabled() {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "OIDC is not enabled")
		return
	}
	// Verify CSRF state.
	stateCookie, err := r.Cookie("oidc_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid state parameter")
		return
	}
	clearOIDCStateCookie(w)

	code := r.URL.Query().Get("code")
	if code == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing authorization code")
		return
	}
	sess, err := s.auth.OIDCCallback(r.Context(), code)
	if err != nil {
		s.logger.Error("OIDC callback error", "err", err)
		writeProblem(w, r, http.StatusUnauthorized, "Unauthorized", "OIDC authentication failed")
		return
	}
	setSessionCookie(w, sess)
	http.Redirect(w, r, "/", http.StatusFound)
}

func setSessionCookie(w http.ResponseWriter, sess *db.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sess.Token,
		Path:     "/",
		Expires:  sess.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

func clearOIDCStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   "oidc_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

func genOIDCState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
