package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// In-memory metric stores
// ---------------------------------------------------------------------------

// grpcRequestCounters tracks cumulative gRPC call counts keyed by method name.
var grpcRequestCounters sync.Map // map[string]*int64

// httpRequestCounters tracks cumulative HTTP request counts keyed by
// "METHOD\x00path\x00status" tuples.
var httpRequestCounters sync.Map // map[string]*int64

// IncrGRPCRequest increments the counter for a gRPC method.
// It is called from the gRPC server's logging interceptors.
func IncrGRPCRequest(method string) {
	v, _ := grpcRequestCounters.LoadOrStore(method, new(int64))
	atomic.AddInt64(v.(*int64), 1)
}

// IncrHTTPRequest increments the counter for an HTTP request.
// It is called from metricsMiddleware.
func IncrHTTPRequest(method, path string, status int) {
	key := fmt.Sprintf("%s\x00%s\x00%d", method, path, status)
	v, _ := httpRequestCounters.LoadOrStore(key, new(int64))
	atomic.AddInt64(v.(*int64), 1)
}

// ---------------------------------------------------------------------------
// /metrics handler
// ---------------------------------------------------------------------------

// handleMetrics returns operational counters and histograms in Prometheus text
// format (version 0.0.4).  This endpoint is intentionally unauthenticated so
// that Prometheus scrapers can reach it without session credentials.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// --- Job counts by status ---
	fmt.Fprintln(w, "# HELP encodeswarmr_jobs_total Number of jobs by status.")
	fmt.Fprintln(w, "# TYPE encodeswarmr_jobs_total gauge")
	for _, st := range []string{"queued", "running", "completed", "failed", "cancelled"} {
		_, total, err := s.store.ListJobs(ctx, db.ListJobsFilter{Status: st, PageSize: 1})
		if err != nil {
			s.logger.Warn("metrics: list jobs", "status", st, "err", err)
			continue
		}
		fmt.Fprintf(w, "encodeswarmr_jobs_total{status=%q} %d\n", st, total)
	}

	// --- Task counts by status and type (derived from job task-count columns) ---
	fmt.Fprintln(w, "# HELP encodeswarmr_tasks_total Number of tasks by status and task_type.")
	fmt.Fprintln(w, "# TYPE encodeswarmr_tasks_total gauge")
	writeTaskCounts(ctx, w, s)

	// --- Agent counts by status ---
	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		s.logger.Warn("metrics: list agents", "err", err)
		return
	}

	agentCounts := map[string]int{}
	for _, a := range agents {
		agentCounts[a.Status]++
	}

	fmt.Fprintln(w, "# HELP encodeswarmr_agents_total Number of registered agents by status.")
	fmt.Fprintln(w, "# TYPE encodeswarmr_agents_total gauge")
	for _, st := range []string{"idle", "running", "offline", "draining", "pending_approval"} {
		fmt.Fprintf(w, "encodeswarmr_agents_total{status=%q} %d\n", st, agentCounts[st])
	}

	// --- Active agents gauge ---
	fmt.Fprintln(w, "# HELP encodeswarmr_active_agents Number of agents currently in running or idle state.")
	fmt.Fprintln(w, "# TYPE encodeswarmr_active_agents gauge")
	active := agentCounts["running"] + agentCounts["idle"]
	fmt.Fprintf(w, "encodeswarmr_active_agents %d\n", active)

	// --- Encoding FPS across all running tasks ---
	fmt.Fprintln(w, "# HELP encodeswarmr_encoding_fps Current encoding FPS aggregated across all running tasks.")
	fmt.Fprintln(w, "# TYPE encodeswarmr_encoding_fps gauge")
	totalFPS := computeRunningFPS(ctx, s)
	fmt.Fprintf(w, "encodeswarmr_encoding_fps %.4f\n", totalFPS)

	// --- Task duration histogram ---
	fmt.Fprintln(w, "# HELP encodeswarmr_task_duration_seconds Histogram of task execution time (started_at to completed_at).")
	fmt.Fprintln(w, "# TYPE encodeswarmr_task_duration_seconds histogram")
	writeDurationHistogram(ctx, w, s)

	// --- Queue wait histogram ---
	fmt.Fprintln(w, "# HELP encodeswarmr_task_queue_wait_seconds Histogram of time tasks spend in pending state before being claimed.")
	fmt.Fprintln(w, "# TYPE encodeswarmr_task_queue_wait_seconds histogram")
	writeQueueWaitHistogram(ctx, w, s)

	// --- Chunk throughput (total output bytes from completed tasks) ---
	fmt.Fprintln(w, "# HELP encodeswarmr_chunk_throughput_bytes Total encoded output bytes across all completed tasks.")
	fmt.Fprintln(w, "# TYPE encodeswarmr_chunk_throughput_bytes counter")
	chunkBytes := computeChunkThroughput(ctx, s)
	fmt.Fprintf(w, "encodeswarmr_chunk_throughput_bytes %d\n", chunkBytes)

	// --- gRPC request counters ---
	fmt.Fprintln(w, "# HELP encodeswarmr_grpc_requests_total Total gRPC calls by method.")
	fmt.Fprintln(w, "# TYPE encodeswarmr_grpc_requests_total counter")
	grpcRequestCounters.Range(func(k, v any) bool {
		method := k.(string)
		count := atomic.LoadInt64(v.(*int64))
		fmt.Fprintf(w, "encodeswarmr_grpc_requests_total{method=%q} %d\n", method, count)
		return true
	})

	// --- HTTP request counters ---
	fmt.Fprintln(w, "# HELP encodeswarmr_http_requests_total Total HTTP requests by method, path, and status code.")
	fmt.Fprintln(w, "# TYPE encodeswarmr_http_requests_total counter")
	httpRequestCounters.Range(func(k, v any) bool {
		var method, path string
		var status int
		fmt.Sscanf(k.(string), "%s\x00%s\x00%d", &method, &path, &status)
		// Sscanf doesn't handle null-byte delimiters well; parse manually.
		method, path, status = parseHTTPKey(k.(string))
		count := atomic.LoadInt64(v.(*int64))
		fmt.Fprintf(w, "encodeswarmr_http_requests_total{method=%q,path=%q,status=\"%d\"} %d\n", method, path, status, count)
		return true
	})
}

// parseHTTPKey extracts method, path, and status from an HTTP counter key
// produced by IncrHTTPRequest.  Fields are separated by null bytes.
func parseHTTPKey(key string) (method, path string, status int) {
	parts := splitNullSep(key)
	if len(parts) < 3 {
		return key, "", 0
	}
	method = parts[0]
	path = parts[1]
	fmt.Sscanf(parts[2], "%d", &status)
	return method, path, status
}

// splitNullSep splits s on null bytes into at most 3 parts.
func splitNullSep(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			out = append(out, s[start:i])
			start = i + 1
			if len(out) == 2 {
				out = append(out, s[start:])
				return out
			}
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// ---------------------------------------------------------------------------
// Task count helpers
// ---------------------------------------------------------------------------

// writeTaskCounts derives task counts from job-level aggregation columns to
// avoid scanning the full tasks table on every scrape.
func writeTaskCounts(ctx context.Context, w http.ResponseWriter, s *Server) {
	type taskKey struct{ status, taskType string }
	counts := map[taskKey]int{}

	for _, jst := range []string{"queued", "running", "completed", "failed"} {
		jobs, _, err := s.store.ListJobs(ctx, db.ListJobsFilter{Status: jst, PageSize: 200})
		if err != nil {
			continue
		}
		for _, j := range jobs {
			counts[taskKey{"pending", "encode"}] += j.TasksPending
			counts[taskKey{"running", "encode"}] += j.TasksRunning
			counts[taskKey{"completed", "encode"}] += j.TasksCompleted
			counts[taskKey{"failed", "encode"}] += j.TasksFailed
		}
	}

	for k, v := range counts {
		fmt.Fprintf(w, "encodeswarmr_tasks_total{status=%q,task_type=%q} %d\n", k.status, k.taskType, v)
	}
}

// ---------------------------------------------------------------------------
// Histogram helpers
// ---------------------------------------------------------------------------

// durationBuckets are the upper bounds (seconds) for task execution histograms.
var durationBuckets = []float64{30, 60, 120, 300, 600, 1200, 1800, 3600, 7200}

// queueBuckets are the upper bounds (seconds) for queue-wait histograms.
var queueBuckets = []float64{5, 15, 30, 60, 120, 300, 600}

// writeDurationHistogram emits a Prometheus histogram for task execution time.
func writeDurationHistogram(ctx context.Context, w http.ResponseWriter, s *Server) {
	jobs, _, err := s.store.ListJobs(ctx, db.ListJobsFilter{Status: "completed", PageSize: 100})
	if err != nil {
		return
	}

	bucketCounts := make([]int64, len(durationBuckets))
	var sum float64
	var count int64

	for _, j := range jobs {
		tasks, err2 := s.store.ListTasksByJob(ctx, j.ID)
		if err2 != nil {
			continue
		}
		for _, t := range tasks {
			if t.StartedAt == nil || t.CompletedAt == nil {
				continue
			}
			dur := t.CompletedAt.Sub(*t.StartedAt).Seconds()
			sum += dur
			count++
			for i, b := range durationBuckets {
				if dur <= b {
					bucketCounts[i]++
				}
			}
		}
	}

	cumulative := int64(0)
	for i, b := range durationBuckets {
		cumulative += bucketCounts[i]
		fmt.Fprintf(w, "encodeswarmr_task_duration_seconds_bucket{le=\"%.0f\"} %d\n", b, cumulative)
	}
	fmt.Fprintf(w, "encodeswarmr_task_duration_seconds_bucket{le=\"+Inf\"} %d\n", count)
	fmt.Fprintf(w, "encodeswarmr_task_duration_seconds_sum %.4f\n", sum)
	fmt.Fprintf(w, "encodeswarmr_task_duration_seconds_count %d\n", count)
}

// writeQueueWaitHistogram emits a Prometheus histogram of task queue wait time.
func writeQueueWaitHistogram(ctx context.Context, w http.ResponseWriter, s *Server) {
	jobs, _, err := s.store.ListJobs(ctx, db.ListJobsFilter{Status: "completed", PageSize: 100})
	if err != nil {
		return
	}

	bucketCounts := make([]int64, len(queueBuckets))
	var sum float64
	var count int64

	for _, j := range jobs {
		tasks, err2 := s.store.ListTasksByJob(ctx, j.ID)
		if err2 != nil {
			continue
		}
		for _, t := range tasks {
			if t.StartedAt == nil {
				continue
			}
			wait := t.StartedAt.Sub(t.CreatedAt).Seconds()
			if wait < 0 {
				wait = 0
			}
			sum += wait
			count++
			for i, b := range queueBuckets {
				if wait <= b {
					bucketCounts[i]++
				}
			}
		}
	}

	cumulative := int64(0)
	for i, b := range queueBuckets {
		cumulative += bucketCounts[i]
		fmt.Fprintf(w, "encodeswarmr_task_queue_wait_seconds_bucket{le=\"%.0f\"} %d\n", b, cumulative)
	}
	fmt.Fprintf(w, "encodeswarmr_task_queue_wait_seconds_bucket{le=\"+Inf\"} %d\n", count)
	fmt.Fprintf(w, "encodeswarmr_task_queue_wait_seconds_sum %.4f\n", sum)
	fmt.Fprintf(w, "encodeswarmr_task_queue_wait_seconds_count %d\n", count)
}

// ---------------------------------------------------------------------------
// Gauge helpers
// ---------------------------------------------------------------------------

// computeRunningFPS sums AvgFPS of all tasks currently in "running" status.
func computeRunningFPS(ctx context.Context, s *Server) float64 {
	jobs, _, err := s.store.ListJobs(ctx, db.ListJobsFilter{Status: "running", PageSize: 50})
	if err != nil {
		return 0
	}
	var total float64
	for _, j := range jobs {
		tasks, err2 := s.store.ListTasksByJob(ctx, j.ID)
		if err2 != nil {
			continue
		}
		for _, t := range tasks {
			if t.Status == "running" && t.AvgFPS != nil {
				total += *t.AvgFPS
			}
		}
	}
	return total
}

// computeChunkThroughput returns the total output bytes across all completed
// tasks (used as a monotonically increasing counter).
func computeChunkThroughput(ctx context.Context, s *Server) int64 {
	jobs, _, err := s.store.ListJobs(ctx, db.ListJobsFilter{Status: "completed", PageSize: 200})
	if err != nil {
		return 0
	}
	var total int64
	for _, j := range jobs {
		tasks, err2 := s.store.ListTasksByJob(ctx, j.ID)
		if err2 != nil {
			continue
		}
		for _, t := range tasks {
			if t.OutputSize != nil {
				total += *t.OutputSize
			}
		}
	}
	return total
}
