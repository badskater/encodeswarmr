package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/db"
)

// handleCreateAPIKey creates a new API key for the authenticated user.
// The plaintext key is returned ONCE in the response; it is never stored.
func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeProblem(w, r, http.StatusUnauthorized, "Unauthorized", "")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "name is required")
		return
	}

	plaintext, keyHash, err := generateAPIKey()
	if err != nil {
		s.logger.Error("generate api key", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	key, err := s.store.CreateAPIKey(r.Context(), db.CreateAPIKeyParams{
		UserID:  claims.UserID,
		Name:    req.Name,
		KeyHash: keyHash,
	})
	if err != nil {
		s.logger.Error("create api key", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Return the plaintext key only in this response.
	writeJSON(w, r, http.StatusCreated, map[string]any{
		"id":         key.ID,
		"user_id":    key.UserID,
		"name":       key.Name,
		"key":        plaintext,
		"created_at": key.CreatedAt,
		"expires_at": key.ExpiresAt,
	})
}

// handleListAPIKeys returns the API keys belonging to the authenticated user.
// The plaintext key and key_hash are never included in the listing.
func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeProblem(w, r, http.StatusUnauthorized, "Unauthorized", "")
		return
	}

	keys, err := s.store.ListAPIKeysByUser(r.Context(), claims.UserID)
	if err != nil {
		s.logger.Error("list api keys", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if keys == nil {
		keys = []*db.APIKey{}
	}
	writeJSON(w, r, http.StatusOK, keys)
}

// handleDeleteAPIKey deletes one of the authenticated user's API keys.
func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.FromContext(r.Context())
	if !ok {
		writeProblem(w, r, http.StatusUnauthorized, "Unauthorized", "")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing id")
		return
	}

	// Verify the key belongs to the requesting user before deleting.
	keys, err := s.store.ListAPIKeysByUser(r.Context(), claims.UserID)
	if err != nil {
		s.logger.Error("list api keys for delete", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	owned := false
	for _, k := range keys {
		if k.ID == id {
			owned = true
			break
		}
	}
	if !owned {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "api key not found")
		return
	}

	if err := s.store.DeleteAPIKey(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "api key not found")
		return
	} else if err != nil {
		s.logger.Error("delete api key", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true})
}

// generateAPIKey creates a random 32-byte hex key and returns both the
// plaintext and its SHA-256 hash (used for storage).
func generateAPIKey() (plaintext, keyHash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	plaintext = hex.EncodeToString(b)
	keyHash = auth.HashAPIKey(plaintext)
	return plaintext, keyHash, nil
}
