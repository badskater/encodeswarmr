package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// TestInitTracer_NoopProvider
// ---------------------------------------------------------------------------

func TestInitTracer_NoopProvider(t *testing.T) {
	// Pass nil exporter → should succeed and return a non-nil shutdown func.
	shutdown, err := InitTracer("test-service", nil, 0)
	if err != nil {
		t.Fatalf("InitTracer(nil exporter) error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("InitTracer returned nil shutdown function")
	}
	// Must not panic.
	shutdown()
}

// ---------------------------------------------------------------------------
// TestInitTracer_SampleRate
// ---------------------------------------------------------------------------

func TestInitTracer_SampleRateDefault(t *testing.T) {
	// sampleRate <= 0 should default to 1.0 without error.
	shutdown, err := InitTracer("test-service", nil, -1)
	if err != nil {
		t.Fatalf("InitTracer with negative sampleRate: %v", err)
	}
	if shutdown == nil {
		t.Fatal("InitTracer returned nil shutdown function")
	}
	shutdown()
}

func TestInitTracer_SampleRatePositive(t *testing.T) {
	shutdown, err := InitTracer("test-service", nil, 0.5)
	if err != nil {
		t.Fatalf("InitTracer with 0.5 sampleRate: %v", err)
	}
	if shutdown == nil {
		t.Fatal("InitTracer returned nil shutdown")
	}
	shutdown()
}

// ---------------------------------------------------------------------------
// TestHTTPMiddleware_NoPanic
// ---------------------------------------------------------------------------

func TestHTTPMiddleware_NoPanic(t *testing.T) {
	// Initialise tracer so the package-level Tracer is not the zero value.
	shutdown, err := InitTracer("test-service", nil, 1.0)
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	defer shutdown()

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := HTTPMiddleware(inner)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	// Must not panic.
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("inner handler was not called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestHTTPMiddleware_ErrorStatusRecorded(t *testing.T) {
	shutdown, err := InitTracer("test-service", nil, 1.0)
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	defer shutdown()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	handler := HTTPMiddleware(inner)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// TestStartJobSpan
// ---------------------------------------------------------------------------

func TestStartJobSpan_ReturnsValidSpan(t *testing.T) {
	shutdown, err := InitTracer("test-service", nil, 1.0)
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	defer shutdown()

	ctx := context.Background()
	spanCtx, end := StartJobSpan(ctx, "job-123", "encode")

	if spanCtx == nil {
		t.Fatal("StartJobSpan returned nil context")
	}
	if end == nil {
		t.Fatal("StartJobSpan returned nil end function")
	}

	// Must not panic.
	end()
}

// ---------------------------------------------------------------------------
// TestStartTaskSpan
// ---------------------------------------------------------------------------

func TestStartTaskSpan_ReturnsValidSpan(t *testing.T) {
	shutdown, err := InitTracer("test-service", nil, 1.0)
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}
	defer shutdown()

	ctx := context.Background()
	spanCtx, end := StartTaskSpan(ctx, "task-456", "job-123")

	if spanCtx == nil {
		t.Fatal("StartTaskSpan returned nil context")
	}
	if end == nil {
		t.Fatal("StartTaskSpan returned nil end function")
	}
	end()
}

// ---------------------------------------------------------------------------
// TestStatusRecorder_DefaultStatus
// ---------------------------------------------------------------------------

func TestStatusRecorder_DefaultStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rr, status: http.StatusOK}

	// Write without explicit WriteHeader — status should remain 200.
	sr.Write([]byte("hello"))

	if sr.status != http.StatusOK {
		t.Errorf("status = %d, want 200", sr.status)
	}
}

func TestStatusRecorder_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rr, status: http.StatusOK}

	sr.WriteHeader(http.StatusCreated)
	if sr.status != http.StatusCreated {
		t.Errorf("status = %d, want 201", sr.status)
	}
}
