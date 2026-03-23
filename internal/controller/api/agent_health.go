package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
)

// agentHealthResponse is the JSON payload returned by GET /api/v1/agents/{id}/health.
type agentHealthResponse struct {
	AgentID    string  `json:"agent_id"`
	Status     string  `json:"status"`
	UptimeSecs int64   `json:"uptime_seconds"`
	CPUUsagePct float32 `json:"cpu_usage_pct"`
	MemUsagePct float32 `json:"memory_usage_pct"`
	GPU         gpuInfo `json:"gpu"`
	Disk        diskInfo `json:"disk"`
	EncodingStats encodingStatsInfo `json:"encoding_stats"`
	LastHeartbeat    *time.Time `json:"last_heartbeat,omitempty"`
	LastTaskCompleted *time.Time `json:"last_task_completed,omitempty"`
}

type gpuInfo struct {
	Name           string  `json:"name"`
	UtilizationPct float32 `json:"utilization_pct"`
}

type diskInfo struct {
	WorkDirFreeGB  float64 `json:"work_dir_free_gb"`
	WorkDirTotalGB float64 `json:"work_dir_total_gb"`
}

type encodingStatsInfo struct {
	TotalTasksCompleted int64   `json:"total_tasks_completed"`
	AvgFPS              float64 `json:"avg_fps"`
	TotalFramesEncoded  int64   `json:"total_frames_encoded"`
	ErrorRatePct        float64 `json:"error_rate_pct"`
}

// handleGetAgentHealth returns a comprehensive health snapshot for a single agent.
// GET /api/v1/agents/{id}/health
//
// Most metrics come from the most-recent agent_metrics row written by the
// heartbeat handler. GPU info is taken from the agent row itself. Encoding
// statistics are aggregated from the tasks table.
func (s *Server) handleGetAgentHealth(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	agent, err := s.store.GetAgentByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "agent not found")
		return
	}
	if err != nil {
		s.logger.Error("get agent for health", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	// Latest metrics sample — use the most-recent row from the last 5 minutes.
	var cpuPct, memPct, gpuPct, diskFreeMiB float32
	metrics, err := s.store.ListAgentMetrics(r.Context(), id, time.Now().Add(-5*time.Minute))
	if err != nil {
		s.logger.Warn("get agent metrics for health", "err", err, "agent_id", id)
	} else if len(metrics) > 0 {
		latest := metrics[len(metrics)-1]
		cpuPct = latest.CPUPct
		memPct = latest.MemPct
		gpuPct = latest.GPUPct
		// disk_free_mib is stored in the heartbeat metrics map on the agent row
		// but the agent_metrics table only stores cpu/gpu/mem. Disk data is not
		// currently persisted; we return 0 here as a placeholder.
		_ = diskFreeMiB
	}

	// Uptime: estimated from last_heartbeat to now.
	var uptimeSecs int64
	if agent.LastHeartbeat != nil {
		uptimeSecs = int64(time.Since(*agent.LastHeartbeat).Seconds())
		// Clamp to 0 (heartbeat in the past means agent is likely offline).
		if uptimeSecs < 0 {
			uptimeSecs = 0
		}
	}

	// Encoding statistics.
	stats, err := s.store.GetAgentEncodingStats(r.Context(), id)
	if err != nil {
		s.logger.Warn("get agent encoding stats for health", "err", err, "agent_id", id)
		stats = &db.AgentEncodingStats{}
	}

	resp := agentHealthResponse{
		AgentID:     agent.ID,
		Status:      agent.Status,
		UptimeSecs:  uptimeSecs,
		CPUUsagePct: cpuPct,
		MemUsagePct: memPct,
		GPU: gpuInfo{
			Name:           agent.GPUVendor + " " + agent.GPUModel,
			UtilizationPct: gpuPct,
		},
		Disk: diskInfo{
			WorkDirFreeGB:  0, // not yet stored in agent_metrics
			WorkDirTotalGB: 0,
		},
		EncodingStats: encodingStatsInfo{
			TotalTasksCompleted: stats.TotalTasksCompleted,
			AvgFPS:              stats.AvgFPS,
			TotalFramesEncoded:  stats.TotalFramesEncoded,
			ErrorRatePct:        stats.ErrorRatePct,
		},
		LastHeartbeat:     agent.LastHeartbeat,
		LastTaskCompleted: stats.LastTaskCompletedAt,
	}

	writeJSON(w, r, http.StatusOK, resp)
}

// handleListAgentRecentTasks returns the 10 most-recent tasks for an agent.
// GET /api/v1/agents/{id}/recent-tasks
func (s *Server) handleListAgentRecentTasks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := s.store.GetAgentByID(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeProblem(w, r, http.StatusNotFound, "Not Found", "agent not found")
			return
		}
		s.logger.Error("get agent for recent tasks", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	tasks, err := s.store.ListRecentTasksByAgent(r.Context(), id, 10)
	if err != nil {
		s.logger.Error("list recent tasks by agent", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}
	if tasks == nil {
		tasks = []*db.Task{}
	}
	writeJSON(w, r, http.StatusOK, tasks)
}
