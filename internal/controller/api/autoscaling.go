package api

import (
	"encoding/json"
	"net/http"
)

// autoScalingResponse is the public representation of auto-scaling settings.
type autoScalingResponse struct {
	Enabled            bool   `json:"enabled"`
	WebhookURL         string `json:"webhook_url"`
	ScaleUpThreshold   int    `json:"scale_up_threshold"`
	ScaleDownThreshold int    `json:"scale_down_threshold"`
	CooldownSeconds    int    `json:"cooldown_seconds"`
}

// handleGetAutoScaling returns the current auto-scaling configuration.
// GET /api/v1/settings/auto-scaling  (admin only)
func (s *Server) handleGetAutoScaling(w http.ResponseWriter, r *http.Request) {
	asc := s.cfg.AutoScaling
	writeJSON(w, r, http.StatusOK, autoScalingResponse{
		Enabled:            asc.Enabled,
		WebhookURL:         asc.WebhookURL,
		ScaleUpThreshold:   asc.ScaleUpThreshold,
		ScaleDownThreshold: asc.ScaleDownThreshold,
		CooldownSeconds:    asc.CooldownSeconds,
	})
}

// handleUpdateAutoScaling updates auto-scaling settings at runtime.
// Changes are applied to the in-memory config; a restart is required to
// persist them to the config file.
// PUT /api/v1/settings/auto-scaling  (admin only)
func (s *Server) handleUpdateAutoScaling(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled            bool   `json:"enabled"`
		WebhookURL         string `json:"webhook_url"`
		ScaleUpThreshold   int    `json:"scale_up_threshold"`
		ScaleDownThreshold int    `json:"scale_down_threshold"`
		CooldownSeconds    int    `json:"cooldown_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return
	}
	if req.ScaleUpThreshold < 0 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			"scale_up_threshold must be >= 0")
		return
	}
	if req.ScaleDownThreshold < 0 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			"scale_down_threshold must be >= 0")
		return
	}
	if req.CooldownSeconds < 0 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "Validation Error",
			"cooldown_seconds must be >= 0")
		return
	}

	s.cfg.AutoScaling.Enabled = req.Enabled
	s.cfg.AutoScaling.WebhookURL = req.WebhookURL
	s.cfg.AutoScaling.ScaleUpThreshold = req.ScaleUpThreshold
	s.cfg.AutoScaling.ScaleDownThreshold = req.ScaleDownThreshold
	s.cfg.AutoScaling.CooldownSeconds = req.CooldownSeconds

	s.logger.Info("auto-scaling config updated",
		"enabled", req.Enabled,
		"scale_up_threshold", req.ScaleUpThreshold,
		"scale_down_threshold", req.ScaleDownThreshold,
		"cooldown_seconds", req.CooldownSeconds,
	)

	writeJSON(w, r, http.StatusOK, autoScalingResponse{
		Enabled:            s.cfg.AutoScaling.Enabled,
		WebhookURL:         s.cfg.AutoScaling.WebhookURL,
		ScaleUpThreshold:   s.cfg.AutoScaling.ScaleUpThreshold,
		ScaleDownThreshold: s.cfg.AutoScaling.ScaleDownThreshold,
		CooldownSeconds:    s.cfg.AutoScaling.CooldownSeconds,
	})
}

// handleTestAutoScalingWebhook sends a test payload to the configured auto-scaling webhook.
// POST /api/v1/settings/auto-scaling/test  (admin only)
func (s *Server) handleTestAutoScalingWebhook(w http.ResponseWriter, r *http.Request) {
	url := s.cfg.AutoScaling.WebhookURL
	if url == "" {
		writeProblem(w, r, http.StatusUnprocessableEntity, "No Webhook URL",
			"set auto_scaling.webhook_url in the controller config or via PUT /api/v1/settings/auto-scaling")
		return
	}

	if err := s.autoScaling.TestWebhook(r.Context(), url); err != nil {
		s.logger.Warn("auto-scaling: test webhook failed", "err", err)
		writeProblem(w, r, http.StatusBadGateway, "Webhook Test Failed", err.Error())
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"ok": true, "url": url})
}
