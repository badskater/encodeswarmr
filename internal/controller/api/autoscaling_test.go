package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/controller/engine"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newAutoScalingTestServer(cfg *config.Config) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	asCfg := cfg.AutoScaling
	srv := &Server{
		store:       &stubStore{},
		logger:      logger,
		cfg:         cfg,
		autoScaling: engine.NewAutoScalingHook(func() config.AutoScalingConfig { return asCfg }, logger),
	}
	return srv
}

func defaultAutoScalingConfig() *config.Config {
	return &config.Config{
		AutoScaling: config.AutoScalingConfig{
			Enabled:            true,
			WebhookURL:         "http://example.com/hook",
			ScaleUpThreshold:   5,
			ScaleDownThreshold: 2,
			CooldownSeconds:    30,
		},
	}
}

// ---------------------------------------------------------------------------
// TestHandleGetAutoScaling
// ---------------------------------------------------------------------------

func TestHandleGetAutoScaling_ReturnsCurrentConfig(t *testing.T) {
	cfg := defaultAutoScalingConfig()
	srv := newAutoScalingTestServer(cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/auto-scaling", nil)
	srv.handleGetAutoScaling(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var body struct {
		Data autoScalingResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		// Response may not be wrapped; try direct decode.
		var direct autoScalingResponse
		rr2 := httptest.NewRecorder()
		srv.handleGetAutoScaling(rr2, req)
		if err2 := json.NewDecoder(rr2.Body).Decode(&direct); err2 != nil {
			t.Fatalf("decode response: %v", err2)
		}
		if direct.ScaleUpThreshold != 5 {
			t.Errorf("scale_up_threshold = %d, want 5", direct.ScaleUpThreshold)
		}
		return
	}
	if body.Data.ScaleUpThreshold != 5 {
		t.Errorf("scale_up_threshold = %d, want 5", body.Data.ScaleUpThreshold)
	}
}

func TestHandleGetAutoScaling_EnabledField(t *testing.T) {
	cfg := defaultAutoScalingConfig()
	cfg.AutoScaling.Enabled = true
	srv := newAutoScalingTestServer(cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/auto-scaling", nil)
	srv.handleGetAutoScaling(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty body")
	}
}

func TestHandleGetAutoScaling_WebhookURLReturned(t *testing.T) {
	cfg := defaultAutoScalingConfig()
	cfg.AutoScaling.WebhookURL = "https://hooks.example.com/scale"
	srv := newAutoScalingTestServer(cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/auto-scaling", nil)
	srv.handleGetAutoScaling(rr, req)

	if !bytes.Contains(rr.Body.Bytes(), []byte("hooks.example.com")) {
		t.Error("response does not contain webhook URL")
	}
}

// ---------------------------------------------------------------------------
// TestHandleUpdateAutoScaling
// ---------------------------------------------------------------------------

func TestHandleUpdateAutoScaling_UpdatesConfig(t *testing.T) {
	cfg := defaultAutoScalingConfig()
	srv := newAutoScalingTestServer(cfg)

	body := `{
		"enabled": true,
		"webhook_url": "https://new.example.com/hook",
		"scale_up_threshold": 10,
		"scale_down_threshold": 4,
		"cooldown_seconds": 60
	}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/auto-scaling", bytes.NewBufferString(body))
	srv.handleUpdateAutoScaling(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	if srv.cfg.AutoScaling.ScaleUpThreshold != 10 {
		t.Errorf("ScaleUpThreshold = %d, want 10", srv.cfg.AutoScaling.ScaleUpThreshold)
	}
	if srv.cfg.AutoScaling.ScaleDownThreshold != 4 {
		t.Errorf("ScaleDownThreshold = %d, want 4", srv.cfg.AutoScaling.ScaleDownThreshold)
	}
	if srv.cfg.AutoScaling.CooldownSeconds != 60 {
		t.Errorf("CooldownSeconds = %d, want 60", srv.cfg.AutoScaling.CooldownSeconds)
	}
	if srv.cfg.AutoScaling.WebhookURL != "https://new.example.com/hook" {
		t.Errorf("WebhookURL = %q, want https://new.example.com/hook", srv.cfg.AutoScaling.WebhookURL)
	}
}

func TestHandleUpdateAutoScaling_InvalidJSON_Returns400(t *testing.T) {
	cfg := defaultAutoScalingConfig()
	srv := newAutoScalingTestServer(cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/auto-scaling",
		bytes.NewBufferString(`{not json`))
	srv.handleUpdateAutoScaling(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleUpdateAutoScaling_NegativeScaleUpThreshold_Returns422(t *testing.T) {
	cfg := defaultAutoScalingConfig()
	srv := newAutoScalingTestServer(cfg)

	body := `{"scale_up_threshold": -1, "scale_down_threshold": 0, "cooldown_seconds": 0}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/auto-scaling",
		bytes.NewBufferString(body))
	srv.handleUpdateAutoScaling(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

func TestHandleUpdateAutoScaling_NegativeScaleDownThreshold_Returns422(t *testing.T) {
	cfg := defaultAutoScalingConfig()
	srv := newAutoScalingTestServer(cfg)

	body := `{"scale_up_threshold": 0, "scale_down_threshold": -1, "cooldown_seconds": 0}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/auto-scaling",
		bytes.NewBufferString(body))
	srv.handleUpdateAutoScaling(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

func TestHandleUpdateAutoScaling_NegativeCooldown_Returns422(t *testing.T) {
	cfg := defaultAutoScalingConfig()
	srv := newAutoScalingTestServer(cfg)

	body := `{"scale_up_threshold": 0, "scale_down_threshold": 0, "cooldown_seconds": -5}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/auto-scaling",
		bytes.NewBufferString(body))
	srv.handleUpdateAutoScaling(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

func TestHandleUpdateAutoScaling_ZeroValuesAccepted(t *testing.T) {
	cfg := defaultAutoScalingConfig()
	srv := newAutoScalingTestServer(cfg)

	body := `{"enabled": false, "webhook_url": "", "scale_up_threshold": 0, "scale_down_threshold": 0, "cooldown_seconds": 0}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/auto-scaling",
		bytes.NewBufferString(body))
	srv.handleUpdateAutoScaling(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (zero values are valid)", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// TestHandleTestAutoScalingWebhook
// ---------------------------------------------------------------------------

func TestHandleTestAutoScalingWebhook_NoURL_Returns422(t *testing.T) {
	cfg := &config.Config{
		AutoScaling: config.AutoScalingConfig{
			WebhookURL: "", // empty
		},
	}
	srv := newAutoScalingTestServer(cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/auto-scaling/test", nil)
	srv.handleTestAutoScalingWebhook(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rr.Code)
	}
}

func TestHandleTestAutoScalingWebhook_ValidURL_Fires(t *testing.T) {
	called := make(chan struct{}, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cfg := &config.Config{
		AutoScaling: config.AutoScalingConfig{
			WebhookURL: ts.URL,
		},
	}
	srv := newAutoScalingTestServer(cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/auto-scaling/test", nil)
	srv.handleTestAutoScalingWebhook(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	select {
	case <-called:
		// webhook was called
	default:
		t.Error("test webhook endpoint was not called")
	}
}

func TestHandleTestAutoScalingWebhook_BadURL_Returns502(t *testing.T) {
	cfg := &config.Config{
		AutoScaling: config.AutoScalingConfig{
			WebhookURL: "http://127.0.0.1:1/nope", // unreachable
		},
	}
	srv := newAutoScalingTestServer(cfg)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/auto-scaling/test", nil)
	srv.handleTestAutoScalingWebhook(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rr.Code)
	}
}
