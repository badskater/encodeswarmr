package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
)

// timeStr formats a time as RFC3339 UTC.
func timeStr(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// webhookResponse is the public representation of a webhook (no secret hash).
type webhookResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Provider  string   `json:"provider"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Enabled   bool     `json:"enabled"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

func toWebhookResponse(wh *db.Webhook) webhookResponse {
	return webhookResponse{
		ID:        wh.ID,
		Name:      wh.Name,
		Provider:  wh.Provider,
		URL:       wh.URL,
		Events:    wh.Events,
		Enabled:   wh.Enabled,
		CreatedAt: timeStr(wh.CreatedAt),
		UpdatedAt: timeStr(wh.UpdatedAt),
	}
}

func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	webhooks, err := s.store.ListWebhooks(r.Context())
	if err != nil {
		s.logger.Error("list webhooks", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	out := make([]webhookResponse, len(webhooks))
	for i, wh := range webhooks {
		out[i] = toWebhookResponse(wh)
	}
	writeJSON(w, r, http.StatusOK, out)
}

func (s *Server) handleGetWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	wh, err := s.store.GetWebhookByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "webhook not found")
			return
		}
		s.logger.Error("get webhook", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, toWebhookResponse(wh))
}

func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string   `json:"name"`
		Provider string   `json:"provider"`
		URL      string   `json:"url"`
		Events   []string `json:"events"`
		Secret   string   `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.Name == "" || req.Provider == "" || req.URL == "" || len(req.Events) == 0 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "name, provider, url, and events are required")
		return
	}
	if !isValidProvider(req.Provider) {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error", "provider must be one of: discord, teams, slack")
		return
	}

	params := db.CreateWebhookParams{
		Name:     req.Name,
		Provider: req.Provider,
		URL:      req.URL,
		Events:   req.Events,
	}
	if req.Secret != "" {
		s := req.Secret
		params.Secret = &s
	}

	wh, err := s.store.CreateWebhook(r.Context(), params)
	if err != nil {
		s.logger.Error("create webhook", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusCreated, toWebhookResponse(wh))
}

func (s *Server) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name    string   `json:"name"`
		URL     string   `json:"url"`
		Events  []string `json:"events"`
		Enabled bool     `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}

	if err := s.store.UpdateWebhook(r.Context(), db.UpdateWebhookParams{
		ID:      id,
		Name:    req.Name,
		URL:     req.URL,
		Events:  req.Events,
		Enabled: req.Enabled,
	}); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "webhook not found")
			return
		}
		s.logger.Error("update webhook", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	wh, err := s.store.GetWebhookByID(r.Context(), id)
	if err != nil {
		s.logger.Error("get webhook after update", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	writeJSON(w, r, http.StatusOK, toWebhookResponse(wh))
}

func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteWebhook(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "webhook not found")
			return
		}
		s.logger.Error("delete webhook", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	wh, err := s.store.GetWebhookByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "webhook not found")
			return
		}
		s.logger.Error("get webhook for test", "err", err)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"event":   "test",
		"message": "Test notification from distributed-encoder",
	})

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(wh.URL, "application/json", bytes.NewReader(payload))

	delivery := db.InsertWebhookDeliveryParams{
		WebhookID: wh.ID,
		Event:     "test",
		Payload:   payload,
		Attempt:   1,
	}

	if err != nil {
		errMsg := err.Error()
		delivery.Success = false
		delivery.ErrorMsg = &errMsg
		if insertErr := s.store.InsertWebhookDelivery(r.Context(), delivery); insertErr != nil {
			s.logger.Error("insert webhook delivery", "err", insertErr)
		}
		writeProblem(w, r, http.StatusBadGateway, "Bad Gateway", "webhook delivery failed")
		return
	}
	defer resp.Body.Close()

	code := resp.StatusCode
	delivery.ResponseCode = &code
	delivery.Success = code >= 200 && code < 300

	if !delivery.Success {
		errMsg := "non-2xx response"
		delivery.ErrorMsg = &errMsg
	}

	if insertErr := s.store.InsertWebhookDelivery(r.Context(), delivery); insertErr != nil {
		s.logger.Error("insert webhook delivery", "err", insertErr)
	}

	if !delivery.Success {
		writeJSON(w, r, http.StatusBadGateway, map[string]any{"ok": false, "status_code": code})
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true, "status_code": code})
}

func (s *Server) handleListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	deliveries, err := s.store.ListWebhookDeliveries(r.Context(), id, limit, offset)
	if err != nil {
		s.logger.Error("list webhook deliveries", "err", err, "webhook_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if deliveries == nil {
		deliveries = []*db.WebhookDelivery{}
	}
	writeJSON(w, r, http.StatusOK, deliveries)
}

func isValidProvider(provider string) bool {
	return provider == "discord" || provider == "teams" || provider == "slack"
}
