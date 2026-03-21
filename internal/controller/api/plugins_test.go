package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/badskater/encodeswarmr/internal/controller/plugins"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newTestServerWithPlugins returns a Server wired with a plugins registry.
func newTestServerWithPlugins(reg *plugins.Registry) *Server {
	srv := newTestServer(&stubStore{})
	srv.plugins = reg
	return srv
}

// ---------------------------------------------------------------------------
// TestHandleListPlugins
// ---------------------------------------------------------------------------

func TestHandleListPlugins(t *testing.T) {
	t.Run("nil registry returns empty array", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		// srv.plugins is nil by default in newTestServer

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins", nil)
		srv.handleListPlugins(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 0 {
			t.Errorf("len(data) = %d, want 0 when registry is nil", len(body.Data))
		}
	})

	t.Run("populated registry returns all plugins", func(t *testing.T) {
		reg := plugins.NewRegistry()
		_ = plugins.RegisterBuiltins(reg)
		srv := newTestServerWithPlugins(reg)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins", nil)
		srv.handleListPlugins(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 4 {
			t.Errorf("len(data) = %d, want 4", len(body.Data))
		}
	})

	t.Run("empty registry returns empty array", func(t *testing.T) {
		reg := plugins.NewRegistry()
		srv := newTestServerWithPlugins(reg)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins", nil)
		srv.handleListPlugins(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 0 {
			t.Errorf("len(data) = %d, want 0", len(body.Data))
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleEnablePlugin
// ---------------------------------------------------------------------------

func TestHandleEnablePlugin(t *testing.T) {
	t.Run("missing plugin name returns 400", func(t *testing.T) {
		reg := plugins.NewRegistry()
		srv := newTestServerWithPlugins(reg)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/plugins//enable", nil)
		// no path value set
		srv.handleEnablePlugin(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("nil registry returns 503", func(t *testing.T) {
		srv := newTestServer(&stubStore{})
		// srv.plugins is nil

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/plugins/x265/enable", nil)
		req.SetPathValue("name", "x265")
		srv.handleEnablePlugin(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rr.Code)
		}
	})

	t.Run("plugin not found returns 404", func(t *testing.T) {
		reg := plugins.NewRegistry()
		srv := newTestServerWithPlugins(reg)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/plugins/nope/enable", nil)
		req.SetPathValue("name", "nope")
		srv.handleEnablePlugin(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("success enables existing plugin", func(t *testing.T) {
		reg := plugins.NewRegistry()
		_ = reg.Register(plugins.Plugin{Name: "x265", Enabled: false})
		srv := newTestServerWithPlugins(reg)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/plugins/x265/enable", nil)
		req.SetPathValue("name", "x265")
		srv.handleEnablePlugin(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["enabled"] != true {
			t.Errorf("data.enabled = %v, want true", body.Data["enabled"])
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleDisablePlugin
// ---------------------------------------------------------------------------

func TestHandleDisablePlugin(t *testing.T) {
	t.Run("missing plugin name returns 400", func(t *testing.T) {
		reg := plugins.NewRegistry()
		srv := newTestServerWithPlugins(reg)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/plugins//disable", nil)
		// no path value set
		srv.handleDisablePlugin(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("plugin not found returns 404", func(t *testing.T) {
		reg := plugins.NewRegistry()
		srv := newTestServerWithPlugins(reg)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/plugins/nope/disable", nil)
		req.SetPathValue("name", "nope")
		srv.handleDisablePlugin(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("success disables existing plugin", func(t *testing.T) {
		reg := plugins.NewRegistry()
		_ = reg.Register(plugins.Plugin{Name: "x265", Enabled: true})
		srv := newTestServerWithPlugins(reg)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/plugins/x265/disable", nil)
		req.SetPathValue("name", "x265")
		srv.handleDisablePlugin(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["enabled"] != false {
			t.Errorf("data.enabled = %v, want false", body.Data["enabled"])
		}
	})

	t.Run("nil registry returns 503", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/plugins/x265/disable", nil)
		req.SetPathValue("name", "x265")
		srv.handleDisablePlugin(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rr.Code)
		}
	})
}
