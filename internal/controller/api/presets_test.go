package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// TestHandleListPresets
// ---------------------------------------------------------------------------

func TestHandleListPresets(t *testing.T) {
	t.Run("returns all 10 presets", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/presets", nil)
		srv.handleListPresets(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if len(body.Data) != 10 {
			t.Errorf("len(data) = %d, want 10", len(body.Data))
		}
	})

	t.Run("each item has required fields", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/presets", nil)
		srv.handleListPresets(rr, req)

		var body struct {
			Data []map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)

		for i, p := range body.Data {
			if p["name"] == "" || p["name"] == nil {
				t.Errorf("data[%d].name is empty", i)
			}
			if p["codec"] == "" || p["codec"] == nil {
				t.Errorf("data[%d].codec is empty", i)
			}
			if p["category"] == "" || p["category"] == nil {
				t.Errorf("data[%d].category is empty", i)
			}
		}
	})

	t.Run("X-Total-Count header equals preset count", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/presets", nil)
		srv.handleListPresets(rr, req)

		xtc := rr.Header().Get("X-Total-Count")
		if xtc != "10" {
			t.Errorf("X-Total-Count = %q, want 10", xtc)
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleGetPreset
// ---------------------------------------------------------------------------

func TestHandleGetPreset(t *testing.T) {
	t.Run("existing preset returns 200 with data", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/presets/1080p+x265+Quality", nil)
		req.SetPathValue("name", "1080p x265 Quality")
		srv.handleGetPreset(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["name"] != "1080p x265 Quality" {
			t.Errorf("data.name = %v, want 1080p x265 Quality", body.Data["name"])
		}
		if body.Data["codec"] != "x265" {
			t.Errorf("data.codec = %v, want x265", body.Data["codec"])
		}
	})

	t.Run("4K HDR preset returns hdr_support=true", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/presets/4K+HDR10+x265+Quality", nil)
		req.SetPathValue("name", "4K HDR10 x265 Quality")
		srv.handleGetPreset(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}

		var body struct {
			Data map[string]any `json:"data"`
		}
		decodeJSON(t, rr, &body)
		if body.Data["hdr_support"] != true {
			t.Errorf("data.hdr_support = %v, want true", body.Data["hdr_support"])
		}
	})

	t.Run("missing preset returns 404", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/presets/does-not-exist", nil)
		req.SetPathValue("name", "does-not-exist")
		srv.handleGetPreset(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("empty name returns 404", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/presets/", nil)
		// No path value set — name will be empty string.
		srv.handleGetPreset(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("all built-in presets are individually retrievable", func(t *testing.T) {
		srv := newTestServer(&stubStore{})

		builtins := []string{
			"4K HDR10 x265 Quality",
			"4K HDR10 x265 Balanced",
			"1080p x265 Quality",
			"1080p x265 Fast",
			"1080p x264 Compatible",
			"Web Optimized H.264",
			"Web Optimized AV1",
			"Archive Lossless",
			"Dolby Vision x265",
			"HDR10+ x265",
		}

		for _, name := range builtins {
			t.Run(name, func(t *testing.T) {
				rr := httptest.NewRecorder()
				// Use a static URL path; the actual preset lookup is driven by
				// the path value, not the raw URL string.
				req := httptest.NewRequest(http.MethodGet, "/api/v1/presets/preset", nil)
				req.SetPathValue("name", name)
				srv.handleGetPreset(rr, req)

				if rr.Code != http.StatusOK {
					t.Fatalf("GET preset %q: status = %d, want 200", name, rr.Code)
				}
			})
		}
	})
}
