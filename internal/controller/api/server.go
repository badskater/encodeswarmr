package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/badskater/distributed-encoder/internal/controller/auth"
	"github.com/badskater/distributed-encoder/internal/controller/config"
	"github.com/badskater/distributed-encoder/internal/controller/ha"
	"github.com/badskater/distributed-encoder/internal/controller/plugins"
	"github.com/badskater/distributed-encoder/internal/controller/webhooks"
	"github.com/badskater/distributed-encoder/internal/db"
)

// Server is the HTTP API server.
type Server struct {
	httpSrv  *http.Server
	store    db.Store
	auth     *auth.Service
	cfg      *config.Config
	logger   *slog.Logger
	webhooks *webhooks.Service
	hub      *Hub
	leader   *ha.Leader
	plugins  *plugins.Registry
}

// New creates and configures a new HTTP API server.
func New(store db.Store, authSvc *auth.Service, cfg *config.Config, logger *slog.Logger, wh *webhooks.Service, ldr *ha.Leader) (*Server, error) {
	// Initialise the plugin registry and register built-in plugins.
	pluginReg := plugins.NewRegistry()
	if err := plugins.RegisterBuiltins(pluginReg); err != nil {
		return nil, fmt.Errorf("api: register builtin plugins: %w", err)
	}

	s := &Server{
		store:    store,
		auth:     authSvc,
		cfg:      cfg,
		logger:   logger,
		webhooks: wh,
		hub:      NewHub(logger),
		leader:   ldr,
		plugins:  pluginReg,
	}

	mux := http.NewServeMux()
	if err := s.registerRoutes(mux); err != nil {
		return nil, err
	}

	// Middleware chain (outermost → innermost):
	//   requestID → security-headers → CORS → rate-limit → metrics → ETag → mux
	handler := s.requestIDMiddleware(
		securityHeadersMiddleware(
			corsMiddleware(cfg.Server.AllowedOrigins,
				rateLimitMiddleware(
					metricsMiddleware(
						etagMiddleware(mux),
					),
				),
			),
		),
	)

	s.httpSrv = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
	return s, nil
}

// Serve starts listening and blocks until ctx is cancelled or a fatal error occurs.
func (s *Server) Serve(ctx context.Context) error {
	// Start WebSocket hub broadcast loop.
	go s.hub.Run(ctx)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return s.httpSrv.Shutdown(shutCtx)
	case err := <-errCh:
		return fmt.Errorf("api: http server: %w", err)
	}
}

// registerRoutes wires all route handlers onto the mux.
func (s *Server) registerRoutes(mux *http.ServeMux) error {
	// Unauthenticated
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("GET /api/v1/openapi.json", s.handleOpenAPISpec)
	mux.HandleFunc("GET /api/v1/ha/status", s.handleHAStatus)

	// Setup wizard — unauthenticated, functional only before first admin exists
	mux.HandleFunc("GET /setup/status", s.handleSetupStatus)
	mux.HandleFunc("POST /setup", s.handleSetup)

	// Auth endpoints (no session required)
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("GET /auth/oidc", s.handleOIDCRedirect)
	mux.HandleFunc("GET /auth/oidc/callback", s.handleOIDCCallback)

	// Agent enrollment — uses one-time token, no session
	mux.HandleFunc("POST /api/v1/agent/enroll", s.handleAgentEnroll)

	// Agent upgrade — no session required (agents use API key, not session)
	mux.HandleFunc("GET /api/v1/agent/upgrade/check", s.handleAgentUpgradeCheck)
	mux.HandleFunc("GET /api/v1/agent/upgrade/{os}/{arch}", s.handleAgentUpgradeDownload)

	viewer := func(h http.HandlerFunc) http.Handler {
		return s.auth.Middleware(auth.RequireRole("viewer", h))
	}
	operator := func(h http.HandlerFunc) http.Handler {
		return s.auth.Middleware(auth.RequireRole("operator", h))
	}
	admin := func(h http.HandlerFunc) http.Handler {
		return s.auth.Middleware(auth.RequireRole("admin", h))
	}

	// --- WebSocket live events ---
	mux.Handle("GET /api/v1/ws", viewer(s.hub.ServeWS))

	// --- Jobs ---
	mux.Handle("GET /api/v1/jobs", viewer(s.handleListJobs))
	mux.Handle("POST /api/v1/jobs", operator(s.handleCreateJob))
	mux.Handle("GET /api/v1/jobs/{id}", viewer(s.handleGetJob))
	mux.Handle("POST /api/v1/jobs/{id}/cancel", operator(s.handleCancelJob))
	mux.Handle("POST /api/v1/jobs/{id}/retry", operator(s.handleRetryJob))
	mux.Handle("GET /api/v1/jobs/{id}/logs", viewer(s.handleListJobLogs))

	// --- Tasks ---
	mux.Handle("GET /api/v1/tasks/{id}", viewer(s.handleGetTask))
	mux.Handle("GET /api/v1/tasks/{id}/logs", viewer(s.handleListTaskLogs))
	mux.Handle("GET /api/v1/tasks/{id}/logs/tail", viewer(s.handleTailTaskLogs))
	mux.Handle("GET /api/v1/tasks/{id}/logs/download", operator(s.handleDownloadTaskLogs))

	// --- Agents ---
	mux.Handle("GET /api/v1/agents", viewer(s.handleListAgents))
	mux.Handle("GET /api/v1/agents/{id}", viewer(s.handleGetAgent))
	mux.Handle("GET /api/v1/agents/{id}/metrics", viewer(s.handleGetAgentMetrics))
	mux.Handle("POST /api/v1/agents/{id}/drain", operator(s.handleDrainAgent))
	mux.Handle("POST /api/v1/agents/{id}/approve", operator(s.handleApproveAgent))
	mux.Handle("POST /api/v1/agents/{id}/upgrade", admin(s.handleRequestAgentUpgrade))

	// --- VNC remote desktop ---
	// WebSocket proxy to the agent's VNC TCP port (noVNC binary framing).
	mux.Handle("GET /api/v1/agents/{id}/vnc", operator(s.handleAgentVNCProxy))
	// Standalone noVNC viewer HTML page — opens in a new browser tab.
	mux.Handle("GET /novnc/{id}", viewer(s.handleNoVNCViewer))

	// --- Agent enrollment tokens ---
	mux.Handle("GET /api/v1/agent-tokens", admin(s.handleListEnrollmentTokens))
	mux.Handle("POST /api/v1/agent-tokens", admin(s.handleCreateEnrollmentToken))
	mux.Handle("DELETE /api/v1/agent-tokens/{id}", admin(s.handleDeleteEnrollmentToken))

	// --- Sources ---
	mux.Handle("GET /api/v1/sources", viewer(s.handleListSources))
	mux.Handle("POST /api/v1/sources", operator(s.handleCreateSource))
	mux.Handle("GET /api/v1/sources/{id}", viewer(s.handleGetSource))
	mux.Handle("GET /api/v1/sources/{id}/scenes", viewer(s.handleGetSourceScenes))
	mux.Handle("POST /api/v1/sources/{id}/encode", operator(s.handleEncodeSource))
	mux.Handle("POST /api/v1/sources/{id}/analyze", operator(s.handleAnalyzeSource))
	mux.Handle("POST /api/v1/sources/{id}/hdr-detect", operator(s.handleHDRDetectSource))
	mux.Handle("PATCH /api/v1/sources/{id}/hdr", operator(s.handleUpdateSourceHDR))
	mux.Handle("DELETE /api/v1/sources/{id}", operator(s.handleDeleteSource))

	// --- Analysis ---
	mux.Handle("POST /api/v1/analysis/scan", operator(s.handleScanAnalysis))
	mux.Handle("GET /api/v1/analysis/{source_id}", viewer(s.handleGetAnalysisResult))
	mux.Handle("GET /api/v1/analysis/{source_id}/all", viewer(s.handleListAnalysisResults))

	// --- Templates ---
	mux.Handle("GET /api/v1/templates", viewer(s.handleListTemplates))
	mux.Handle("GET /api/v1/templates/{id}", viewer(s.handleGetTemplate))
	mux.Handle("POST /api/v1/templates", admin(s.handleCreateTemplate))
	mux.Handle("PUT /api/v1/templates/{id}", admin(s.handleUpdateTemplate))
	mux.Handle("DELETE /api/v1/templates/{id}", admin(s.handleDeleteTemplate))

	// --- Variables ---
	mux.Handle("GET /api/v1/variables", viewer(s.handleListVariables))
	mux.Handle("GET /api/v1/variables/{name}", viewer(s.handleGetVariable))
	mux.Handle("PUT /api/v1/variables/{name}", admin(s.handleUpsertVariable))
	mux.Handle("DELETE /api/v1/variables/{id}", admin(s.handleDeleteVariable))

	// --- Path Mappings (UNC ↔ Linux NFS mount translation) ---
	mux.Handle("GET /api/v1/path-mappings", viewer(s.handleListPathMappings))
	mux.Handle("POST /api/v1/path-mappings", admin(s.handleCreatePathMapping))
	mux.Handle("GET /api/v1/path-mappings/{id}", viewer(s.handleGetPathMapping))
	mux.Handle("PUT /api/v1/path-mappings/{id}", admin(s.handleUpdatePathMapping))
	mux.Handle("DELETE /api/v1/path-mappings/{id}", admin(s.handleDeletePathMapping))

	// --- Webhooks ---
	mux.Handle("GET /api/v1/webhooks", admin(s.handleListWebhooks))
	mux.Handle("GET /api/v1/webhooks/{id}", admin(s.handleGetWebhook))
	mux.Handle("POST /api/v1/webhooks", admin(s.handleCreateWebhook))
	mux.Handle("PUT /api/v1/webhooks/{id}", admin(s.handleUpdateWebhook))
	mux.Handle("DELETE /api/v1/webhooks/{id}", admin(s.handleDeleteWebhook))
	mux.Handle("POST /api/v1/webhooks/{id}/test", admin(s.handleTestWebhook))
	mux.Handle("GET /api/v1/webhooks/{id}/deliveries", admin(s.handleListWebhookDeliveries))

	// --- Users ---
	mux.Handle("GET /api/v1/users", admin(s.handleListUsers))
	mux.Handle("POST /api/v1/users", admin(s.handleCreateUser))
	mux.Handle("DELETE /api/v1/users/{id}", admin(s.handleDeleteUser))
	mux.Handle("PUT /api/v1/users/{id}/role", admin(s.handleUpdateUserRole))
	mux.Handle("GET /api/v1/users/me", viewer(s.handleGetMe))

	// --- API Keys ---
	mux.Handle("POST /api/v1/api-keys", viewer(s.handleCreateAPIKey))
	mux.Handle("GET /api/v1/api-keys", viewer(s.handleListAPIKeys))
	mux.Handle("DELETE /api/v1/api-keys/{id}", viewer(s.handleDeleteAPIKey))

	// --- Notification Preferences (per-user) ---
	mux.Handle("GET /api/v1/me/notifications", viewer(s.handleGetNotificationPrefs))
	mux.Handle("PUT /api/v1/me/notifications", viewer(s.handleUpdateNotificationPrefs))

	// --- Audit Log ---
	mux.Handle("GET /api/v1/audit-log", admin(s.handleListAuditLog))

	// --- Encoding Presets ---
	mux.Handle("GET /api/v1/presets", viewer(s.handleListPresets))
	mux.Handle("GET /api/v1/presets/{name}", viewer(s.handleGetPreset))

	// --- Cost Estimation ---
	mux.Handle("POST /api/v1/estimate", viewer(s.handleEstimate))

	// --- Schedules ---
	mux.Handle("GET /api/v1/schedules", viewer(s.handleListSchedules))
	mux.Handle("POST /api/v1/schedules", admin(s.handleCreateSchedule))
	mux.Handle("GET /api/v1/schedules/{id}", viewer(s.handleGetSchedule))
	mux.Handle("PUT /api/v1/schedules/{id}", admin(s.handleUpdateSchedule))
	mux.Handle("DELETE /api/v1/schedules/{id}", admin(s.handleDeleteSchedule))

	// --- Plugins ---
	mux.Handle("GET /api/v1/plugins", viewer(s.handleListPlugins))
	mux.Handle("PUT /api/v1/plugins/{name}/enable", admin(s.handleEnablePlugin))
	mux.Handle("PUT /api/v1/plugins/{name}/disable", admin(s.handleDisablePlugin))

	// --- Dashboard metrics ---
	mux.Handle("GET /api/v1/metrics/throughput", viewer(s.handleThroughput))
	mux.Handle("GET /api/v1/metrics/queue", viewer(s.handleQueueSummary))
	mux.Handle("GET /api/v1/metrics/activity", viewer(s.handleRecentActivity))

	// Static UI — must be last so API routes take precedence.
	staticH, err := s.staticHandler()
	if err != nil {
		return err
	}
	mux.Handle("/", staticH)
	return nil
}

// requestIDMiddleware injects a correlation ID into each request and response.
func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(requestIDHeader)
		if reqID == "" {
			reqID = genID()
		}
		r.Header.Set(requestIDHeader, reqID)
		w.Header().Set(requestIDHeader, reqID)
		next.ServeHTTP(w, r)
	})
}

// genID returns a 16-hex-character random ID for request correlation.
func genID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
