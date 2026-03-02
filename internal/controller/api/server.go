package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/badskater/distributed-encoder/internal/controller/auth"
	"github.com/badskater/distributed-encoder/internal/controller/config"
	"github.com/badskater/distributed-encoder/internal/db"
)

// Server is the HTTP API server.
type Server struct {
	httpSrv *http.Server
	store   db.Store
	auth    *auth.Service
	cfg     *config.Config
	logger  *slog.Logger
}

// New creates and configures a new HTTP API server.
func New(store db.Store, authSvc *auth.Service, cfg *config.Config, logger *slog.Logger) *Server {
	s := &Server{
		store:  store,
		auth:   authSvc,
		cfg:    cfg,
		logger: logger,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpSrv = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      s.requestIDMiddleware(mux),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
	return s
}

// Serve starts listening and blocks until ctx is cancelled or a fatal error occurs.
func (s *Server) Serve(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	select {
	case <-ctx.Done():
		return s.httpSrv.Shutdown(context.Background())
	case err := <-errCh:
		return fmt.Errorf("api: http server: %w", err)
	}
}

// registerRoutes wires all route handlers onto the mux.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Unauthenticated
	mux.HandleFunc("GET /health", s.handleHealth)

	// Auth endpoints (no session required)
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("GET /auth/oidc", s.handleOIDCRedirect)
	mux.HandleFunc("GET /auth/oidc/callback", s.handleOIDCCallback)
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
