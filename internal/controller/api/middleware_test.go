package api

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// corsMiddleware
// ---------------------------------------------------------------------------

func TestCORSMiddleware(t *testing.T) {
	origins := []string{"http://allowed.example.com"}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := corsMiddleware(origins, next)

	t.Run("allowed origin sets ACAO header", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "http://allowed.example.com")
		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") != "http://allowed.example.com" {
			t.Errorf("ACAO header = %q, want allowed origin", rr.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("disallowed origin does not set ACAO header", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "http://evil.example.com")
		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Origin") == "http://evil.example.com" {
			t.Error("ACAO header should not be set for disallowed origin")
		}
	})

	t.Run("OPTIONS preflight returns 204", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Header.Set("Origin", "http://allowed.example.com")
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rr.Code)
		}
	})

	t.Run("non-OPTIONS passes through to handler", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}
	})

	t.Run("sets standard CORS headers on all requests", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(rr, req)

		if rr.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Error("Access-Control-Allow-Methods header not set")
		}
		if rr.Header().Get("Access-Control-Allow-Headers") == "" {
			t.Error("Access-Control-Allow-Headers header not set")
		}
		if rr.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Error("Access-Control-Allow-Credentials not set to true")
		}
	})
}

// ---------------------------------------------------------------------------
// rateLimitMiddleware
// ---------------------------------------------------------------------------

func TestRateLimitMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// The rate-limit middleware is now a method on Server so it can access the
	// store for per-key limits.  Use a minimal server instance for tests.
	srv := newTestServer(&stubStore{})
	handler := srv.rateLimitMiddleware(next)

	t.Run("normal requests pass through", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}
	})

	t.Run("requests with no port in RemoteAddr still handled", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.2:9999"
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// responseBuffer (WriteHeader / Write)
// ---------------------------------------------------------------------------

func TestResponseBuffer(t *testing.T) {
	t.Run("WriteHeader stores status code", func(t *testing.T) {
		underlying := httptest.NewRecorder()
		buf := &responseBuffer{ResponseWriter: underlying, status: http.StatusOK}
		buf.WriteHeader(http.StatusCreated)
		if buf.status != http.StatusCreated {
			t.Errorf("status = %d, want 201", buf.status)
		}
	})

	t.Run("Write accumulates bytes", func(t *testing.T) {
		underlying := httptest.NewRecorder()
		buf := &responseBuffer{ResponseWriter: underlying, status: http.StatusOK}

		n, err := buf.Write([]byte("hello"))
		if err != nil {
			t.Fatalf("Write: %v", err)
		}
		if n != 5 {
			t.Errorf("n = %d, want 5", n)
		}
		buf.Write([]byte(" world"))
		if string(buf.body) != "hello world" {
			t.Errorf("body = %q, want 'hello world'", string(buf.body))
		}
	})
}

// ---------------------------------------------------------------------------
// etagMiddleware
// ---------------------------------------------------------------------------

func TestEtagMiddleware(t *testing.T) {
	body := []byte(`{"data":"test"}`)
	hash := sha256.Sum256(body)
	expectedEtag := fmt.Sprintf(`"%x"`, hash)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})
	handler := etagMiddleware(next)

	t.Run("GET /api/v1/ adds ETag header", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rr.Code)
		}
		if rr.Header().Get("ETag") != expectedEtag {
			t.Errorf("ETag = %q, want %q", rr.Header().Get("ETag"), expectedEtag)
		}
	})

	t.Run("GET /api/v1/ with matching If-None-Match returns 304", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
		req.Header.Set("If-None-Match", expectedEtag)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotModified {
			t.Errorf("status = %d, want 304", rr.Code)
		}
	})

	t.Run("POST /api/v1/ bypasses etag logic", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", nil)
		handler.ServeHTTP(rr, req)

		// no ETag header for non-GET
		if rr.Header().Get("ETag") != "" {
			t.Errorf("unexpected ETag header on POST: %q", rr.Header().Get("ETag"))
		}
	})

	t.Run("GET non-api path bypasses etag logic", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
		handler.ServeHTTP(rr, req)

		if rr.Header().Get("ETag") != "" {
			t.Errorf("unexpected ETag on non-api path: %q", rr.Header().Get("ETag"))
		}
	})

	t.Run("non-200 response written as-is", func(t *testing.T) {
		notFoundHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		})
		h := etagMiddleware(notFoundHandler)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rr.Code)
		}
		if rr.Header().Get("ETag") != "" {
			t.Errorf("unexpected ETag on non-200 response: %q", rr.Header().Get("ETag"))
		}
	})
}

// ---------------------------------------------------------------------------
// requestIDMiddleware
// ---------------------------------------------------------------------------

func TestRequestIDMiddleware(t *testing.T) {
	srv := newTestServer(&stubStore{})

	t.Run("missing request ID is generated", func(t *testing.T) {
		var capturedID string
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedID = r.Header.Get(requestIDHeader)
			w.WriteHeader(http.StatusOK)
		})
		handler := srv.requestIDMiddleware(inner)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(rr, req)

		if capturedID == "" {
			t.Error("expected request ID to be generated")
		}
		if rr.Header().Get(requestIDHeader) == "" {
			t.Error("expected X-Request-ID response header")
		}
	})

	t.Run("existing request ID is preserved", func(t *testing.T) {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		handler := srv.requestIDMiddleware(inner)

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(requestIDHeader, "my-custom-id")
		handler.ServeHTTP(rr, req)

		if rr.Header().Get(requestIDHeader) != "my-custom-id" {
			t.Errorf("response X-Request-ID = %q, want my-custom-id", rr.Header().Get(requestIDHeader))
		}
	})
}

// ---------------------------------------------------------------------------
// genID
// ---------------------------------------------------------------------------

func TestGenID(t *testing.T) {
	t.Run("returns 16 hex chars", func(t *testing.T) {
		id := genID()
		if len(id) != 16 {
			t.Errorf("len(id) = %d, want 16", len(id))
		}
	})

	t.Run("consecutive calls return different IDs", func(t *testing.T) {
		a := genID()
		b := genID()
		if a == b {
			t.Error("expected different IDs from consecutive calls")
		}
	})

	t.Run("returns lowercase hex", func(t *testing.T) {
		id := genID()
		if strings.ToLower(id) != id {
			t.Errorf("id %q is not lowercase hex", id)
		}
	})
}
