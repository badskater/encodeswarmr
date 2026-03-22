// Package tracing provides lightweight OpenTelemetry initialisation for the
// controller.  It uses the otel SDK (already a transitive dependency) with a
// simple span exporter.  When an OTLP endpoint is configured the application
// can swap in a real exporter; by default a no-op provider is used so no
// additional dependencies are required at this time.
//
// Usage:
//
//	shutdown, err := tracing.InitTracer("encodeswarmr-controller", 0.1)
//	if err != nil { ... }
//	defer shutdown()
package tracing

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Tracer is the package-level tracer for the controller.
var Tracer = otel.Tracer("encodeswarmr")

// SpanExporter is a named alias kept for documentation purposes: callers can
// inject any sdktrace.SpanExporter (e.g. an OTLP exporter added later).
type SpanExporter = sdktrace.SpanExporter

// InitTracer configures a TracerProvider using the supplied exporter.
// Pass nil for exporter to use a no-op provider that discards all spans.
// sampleRate is a fraction in [0,1]; values ≤0 default to 1.0.
//
// The returned shutdown function must be called on application exit to flush
// remaining spans.
func InitTracer(serviceName string, exporter SpanExporter, sampleRate float64) (shutdown func(), err error) {
	if sampleRate <= 0 {
		sampleRate = 1.0
	}

	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			// semconv.ServiceName key is "service.name" per OTel spec.
			attribute.String("service.name", serviceName),
		),
	)
	if err != nil {
		// Non-fatal: fall back to default resource.
		res = resource.Default()
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(sampleRate)),
	}

	if exporter != nil {
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	tp := sdktrace.NewTracerProvider(opts...)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Update package-level tracer with new provider.
	Tracer = tp.Tracer(serviceName)

	return func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = tp.Shutdown(shutCtx)
	}, nil
}

// HTTPMiddleware wraps an HTTP handler and creates a span for each request.
// Span attributes include method, path, and response status.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		spanName := r.Method + " " + r.URL.Path
		ctx, span := Tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
			),
		)
		defer span.End()

		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))

		span.SetAttributes(attribute.Int("http.status_code", rw.status))
		if rw.status >= 500 {
			span.RecordError(nil)
		}
	})
}

// statusRecorder wraps http.ResponseWriter to capture the response status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// StartJobSpan starts a span for a job expansion operation.  The returned
// context carries the span, and the end function must be called when done.
func StartJobSpan(ctx context.Context, jobID, jobType string) (context.Context, func()) {
	ctx, span := Tracer.Start(ctx, "job.expand",
		trace.WithAttributes(
			attribute.String("job.id", jobID),
			attribute.String("job.type", jobType),
		),
	)
	return ctx, func() { span.End() }
}

// StartTaskSpan starts a span for the lifecycle of a single task.
func StartTaskSpan(ctx context.Context, taskID, jobID string) (context.Context, func()) {
	ctx, span := Tracer.Start(ctx, "task.lifecycle",
		trace.WithAttributes(
			attribute.String("task.id", taskID),
			attribute.String("job.id", jobID),
		),
	)
	return ctx, func() { span.End() }
}
