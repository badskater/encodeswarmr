package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(requestIDHeader, "req-abc-123")

	type payload struct {
		Name string `json:"name"`
	}
	writeJSON(rr, req, http.StatusCreated, payload{Name: "hello"})

	// Status code.
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	// Content-Type.
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}

	// Body structure.
	var body struct {
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
		Meta struct {
			RequestID string `json:"request_id"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.Name != "hello" {
		t.Errorf("data.name = %q, want %q", body.Data.Name, "hello")
	}
	if body.Meta.RequestID != "req-abc-123" {
		t.Errorf("meta.request_id = %q, want %q", body.Meta.RequestID, "req-abc-123")
	}
}

func TestWriteProblem(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
	req.Header.Set(requestIDHeader, "req-problem-456")

	writeProblem(rr, req, http.StatusNotFound, "Not Found", "job not found")

	// Status code.
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}

	// Content-Type.
	ct := rr.Header().Get("Content-Type")
	if ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/problem+json")
	}

	// Body fields.
	var body struct {
		Type      string `json:"type"`
		Title     string `json:"title"`
		Status    int    `json:"status"`
		Detail    string `json:"detail"`
		Instance  string `json:"instance"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", body.Status, http.StatusNotFound)
	}
	if body.Title != "Not Found" {
		t.Errorf("title = %q, want %q", body.Title, "Not Found")
	}
	if body.Detail != "job not found" {
		t.Errorf("detail = %q, want %q", body.Detail, "job not found")
	}
	if body.Instance != "/api/v1/jobs" {
		t.Errorf("instance = %q, want %q", body.Instance, "/api/v1/jobs")
	}
	if body.RequestID != "req-problem-456" {
		t.Errorf("request_id = %q, want %q", body.RequestID, "req-problem-456")
	}

	// type URI should contain the correct slug.
	wantType := "https://encodeswarmr.dev/errors/not-found"
	if body.Type != wantType {
		t.Errorf("type = %q, want %q", body.Type, wantType)
	}
}

func TestWriteCollection(t *testing.T) {
	t.Run("with next cursor", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		req.Header.Set(requestIDHeader, "req-coll-1")

		items := []string{"a", "b"}
		writeCollection(rr, req, items, 42, "cursor-next")

		// Status code.
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		// X-Total-Count header.
		xtc := rr.Header().Get("X-Total-Count")
		if xtc != "42" {
			t.Errorf("X-Total-Count = %q, want %q", xtc, "42")
		}

		// Body structure.
		var body struct {
			Data []string       `json:"data"`
			Meta map[string]any `json:"meta"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.Data) != 2 {
			t.Fatalf("len(data) = %d, want 2", len(body.Data))
		}

		// meta.request_id
		rid, ok := body.Meta["request_id"].(string)
		if !ok || rid != "req-coll-1" {
			t.Errorf("meta.request_id = %v, want %q", body.Meta["request_id"], "req-coll-1")
		}

		// meta.total_count (JSON numbers decode as float64)
		tc, ok := body.Meta["total_count"].(float64)
		if !ok || tc != 42 {
			t.Errorf("meta.total_count = %v, want 42", body.Meta["total_count"])
		}

		// meta.next_cursor present.
		nc, ok := body.Meta["next_cursor"].(string)
		if !ok || nc != "cursor-next" {
			t.Errorf("meta.next_cursor = %v, want %q", body.Meta["next_cursor"], "cursor-next")
		}
	})

	t.Run("without next cursor", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		req.Header.Set(requestIDHeader, "req-coll-2")

		writeCollection(rr, req, []string{}, 0, "")

		var body struct {
			Meta map[string]any `json:"meta"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		// meta.next_cursor must NOT be present.
		if _, exists := body.Meta["next_cursor"]; exists {
			t.Errorf("meta.next_cursor should not be present when cursor is empty, got %v", body.Meta["next_cursor"])
		}
	})
}

func TestProblemSlug(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{http.StatusUnauthorized, "unauthorized"},
		{http.StatusForbidden, "forbidden"},
		{http.StatusNotFound, "not-found"},
		{http.StatusUnprocessableEntity, "validation"},
		{http.StatusBadRequest, "bad-request"},
		{http.StatusInternalServerError, "internal"},
		{http.StatusServiceUnavailable, "internal"}, // unknown status falls back to internal
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			got := problemSlug(tt.status)
			if got != tt.want {
				t.Errorf("problemSlug(%d) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}
