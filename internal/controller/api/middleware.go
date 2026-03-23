package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// metricsMiddleware wraps an HTTP handler and records each request in the
// in-memory HTTP counter that is exposed via /metrics.  It captures the
// response status code written by the downstream handler.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip counting /metrics itself to avoid a feedback loop.
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		rw := &statusCapture{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		IncrHTTPRequest(r.Method, r.URL.Path, rw.status)
	})
}

// statusCapture wraps ResponseWriter so we can read the status code after the
// downstream handler has written it.
type statusCapture struct {
	http.ResponseWriter
	status  int
	written bool
}

func (sc *statusCapture) WriteHeader(code int) {
	if !sc.written {
		sc.status = code
		sc.written = true
	}
	sc.ResponseWriter.WriteHeader(code)
}

func (sc *statusCapture) Write(b []byte) (int, error) {
	if !sc.written {
		sc.status = http.StatusOK
		sc.written = true
	}
	return sc.ResponseWriter.Write(b)
}

// corsMiddleware sets CORS headers based on the provided allowed origins and
// handles preflight OPTIONS requests.
func corsMiddleware(origins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowed[o] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowed[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware enforces per-IP and per-API-key token-bucket rate limiting.
//
// Default limits (per-IP): 200 req/s, burst 400.
// When a request authenticates via a Bearer API key, the key's own rate_limit
// field is used instead of the global per-IP default.
//
// Idle entries are evicted after 10 minutes to prevent unbounded memory growth.
func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	type entry struct {
		limiter  *rate.Limiter
		lastSeen atomic.Int64 // UnixNano
	}

	var ipClients  sync.Map // map[string]*entry  — keyed by client IP
	var keyClients sync.Map // map[string]*entry  — keyed by SHA-256 of API key

	// Background goroutine evicts entries not seen in the last 10 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-10 * time.Minute).UnixNano()
			for _, m := range [](*sync.Map){&ipClients, &keyClients} {
				m.Range(func(k, v any) bool {
					if v.(*entry).lastSeen.Load() < cutoff {
						m.Delete(k)
					}
					return true
				})
			}
		}
	}()

	rejectTooMany := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":   "https://encodeswarmr.dev/errors/rate-limit",
			"title":  "Too Many Requests",
			"status": 429,
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now().UnixNano()

		// Check for API key Bearer token.
		if hdr := r.Header.Get("Authorization"); strings.HasPrefix(hdr, "Bearer ") {
			plaintext := strings.TrimPrefix(hdr, "Bearer ")
			sum := sha256.Sum256([]byte(plaintext))
			keyHash := fmt.Sprintf("%x", sum)

			// Look up the key's custom rate limit from DB (non-blocking best-effort).
			apiKey, err := s.store.GetAPIKeyByHashWithRateLimit(r.Context(), keyHash)
			rps := 200 // default
			if err == nil && apiKey != nil && apiKey.RateLimit > 0 {
				rps = apiKey.RateLimit
			}

			// Create or reuse per-key limiter.
			newEntry := &entry{limiter: rate.NewLimiter(rate.Limit(rps), rps*2)}
			actual, _ := keyClients.LoadOrStore(keyHash, newEntry)
			e := actual.(*entry)
			e.lastSeen.Store(now)

			if !e.limiter.Allow() {
				rejectTooMany(w)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Fall back to per-IP limiting.
		ip := r.RemoteAddr
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}

		newEntry := &entry{limiter: rate.NewLimiter(200, 400)}
		actual, _ := ipClients.LoadOrStore(ip, newEntry)
		e := actual.(*entry)
		e.lastSeen.Store(now)

		if !e.limiter.Allow() {
			rejectTooMany(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}


// responseBuffer captures the status code and body written by downstream
// handlers so that etagMiddleware can compute a hash before flushing.
type responseBuffer struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (rb *responseBuffer) WriteHeader(code int) {
	rb.status = code
}

func (rb *responseBuffer) Write(b []byte) (int, error) {
	rb.body = append(rb.body, b...)
	return len(b), nil
}

// securityHeadersMiddleware sets standard HTTP security headers on every
// response to mitigate common web vulnerabilities (XSS, clickjacking, MIME
// sniffing, etc.).  HSTS is only added when the request was received over TLS.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws: wss:")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// etagMiddleware computes a SHA-256 ETag for GET /api/v1/* responses that
// return 200 OK and returns 304 Not Modified when the client already has the
// current version.
func etagMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		buf := &responseBuffer{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(buf, r)

		if buf.status != http.StatusOK {
			w.WriteHeader(buf.status)
			_, _ = w.Write(buf.body)
			return
		}

		hash := sha256.Sum256(buf.body)
		etag := fmt.Sprintf(`"%x"`, hash)

		if r.Header.Get("If-None-Match") == etag {
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("ETag", etag)
		w.WriteHeader(buf.status)
		_, _ = w.Write(buf.body)
	})
}
