package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	pb "github.com/badskater/encodeswarmr/internal/proto/encoderv1"
)

// healthResponse is returned by the /health endpoint.
type healthResponse struct {
	Status              string `json:"status"`
	ControllerConnected bool   `json:"controller_connected"`
	CurrentJob          string `json:"current_job"`
	Uptime              string `json:"uptime"`
	State               string `json:"state"`
}

// stateLabel returns a human-readable label for the agent state.
func stateLabel(s pb.AgentState) string {
	switch s {
	case pb.AgentState_AGENT_STATE_IDLE:
		return "IDLE"
	case pb.AgentState_AGENT_STATE_BUSY:
		return "BUSY"
	case pb.AgentState_AGENT_STATE_DRAINING:
		return "DRAINING"
	case pb.AgentState_AGENT_STATE_OFFLINE:
		return "OFFLINE"
	default:
		return "UNKNOWN"
	}
}

// stateGauge returns a numeric gauge value for the agent state.
func stateGauge(s pb.AgentState) int {
	switch s {
	case pb.AgentState_AGENT_STATE_IDLE:
		return 1
	case pb.AgentState_AGENT_STATE_BUSY:
		return 2
	default:
		return 0
	}
}

// startDebugServer starts a debug HTTP server on localhost:9080 that exposes
// /health and /metrics endpoints. It runs until ctx is cancelled.
func startDebugServer(ctx context.Context, r *runner, log *slog.Logger) {
	startedAt := time.Now()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		state := r.state
		taskID := r.currentTaskID
		r.mu.Unlock()

		resp := healthResponse{
			Status:              "running",
			ControllerConnected: r.conn != nil,
			CurrentJob:          taskID,
			Uptime:              time.Since(startedAt).Round(time.Second).String(),
			State:               stateLabel(state),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		state := r.state
		r.mu.Unlock()

		uptimeSec := time.Since(startedAt).Seconds()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "# HELP agent_state Current agent state (1=idle, 2=busy)\n")
		fmt.Fprintf(w, "# TYPE agent_state gauge\n")
		fmt.Fprintf(w, "agent_state %d\n", stateGauge(state))
		fmt.Fprintf(w, "# HELP agent_uptime_seconds Agent uptime in seconds\n")
		fmt.Fprintf(w, "# TYPE agent_uptime_seconds gauge\n")
		fmt.Fprintf(w, "agent_uptime_seconds %.0f\n", uptimeSec)
	})

	srv := &http.Server{
		Addr:    "localhost:9080",
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Info("debug HTTP server starting", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("debug HTTP server error", "error", err)
	}
}
