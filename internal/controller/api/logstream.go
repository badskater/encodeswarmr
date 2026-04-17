package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// Prometheus-style counters and gauges for the log-stream hub.
// ---------------------------------------------------------------------------

// logStreamRejectedUser counts connections rejected because the per-user cap
// was exceeded.
var logStreamRejectedUser atomic.Int64

// logStreamRejectedIP counts connections rejected because the per-IP cap was
// exceeded.
var logStreamRejectedIP atomic.Int64

// logStreamActiveTotal is a gauge tracking the current total number of active
// log-stream WebSocket connections across all tasks and users.
var logStreamActiveTotal atomic.Int64

// ---------------------------------------------------------------------------
// logStreamHub
// ---------------------------------------------------------------------------

// logStreamHub manages per-task WebSocket subscriptions for task log streaming.
// When new task log entries arrive (via gRPC StreamLogs), PublishTaskLog
// broadcasts them to every subscriber watching that task ID.
type logStreamHub struct {
	mu      sync.Mutex
	subs    map[string]map[*logStreamConn]struct{} // taskID → set of connections
	byUser  map[string]int                         // userID → active count
	byIP    map[string]int                         // client IP → active count
	cfg     config.LogStreamConfig
}

type logStreamConn struct {
	conn      *websocket.Conn
	send      chan []byte
	closeOnce sync.Once
}

func (c *logStreamConn) close() {
	c.closeOnce.Do(func() {
		if c.conn != nil {
			c.conn.Close()
		}
	})
}

// newLogStreamHub creates an idle hub.  No goroutine is needed — sends are
// direct (non-blocking) from PublishTaskLog.
func newLogStreamHub(cfg config.LogStreamConfig) *logStreamHub {
	return &logStreamHub{
		subs:   make(map[string]map[*logStreamConn]struct{}),
		byUser: make(map[string]int),
		byIP:   make(map[string]int),
		cfg:    cfg,
	}
}

// subscribe registers conn as a subscriber for taskID under the given userID
// and clientIP, enforcing per-user and per-IP caps.  Returns (cleanup, true)
// on success, or (nil, false) when a cap is exceeded.
//
// reason is set to "user_limit" or "ip_limit" when the cap is hit (for
// metrics and caller logging).
func (h *logStreamHub) subscribe(taskID, userID, clientIP string, conn *logStreamConn) (cleanup func(), ok bool, reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cfg.MaxPerUser > 0 && h.byUser[userID] >= h.cfg.MaxPerUser {
		return nil, false, "user_limit"
	}
	if h.cfg.MaxPerIP > 0 && h.byIP[clientIP] >= h.cfg.MaxPerIP {
		return nil, false, "ip_limit"
	}

	if h.subs[taskID] == nil {
		h.subs[taskID] = make(map[*logStreamConn]struct{})
	}
	h.subs[taskID][conn] = struct{}{}
	h.byUser[userID]++
	h.byIP[clientIP]++
	logStreamActiveTotal.Add(1)

	return func() {
		h.mu.Lock()
		delete(h.subs[taskID], conn)
		if len(h.subs[taskID]) == 0 {
			delete(h.subs, taskID)
		}
		h.byUser[userID]--
		if h.byUser[userID] == 0 {
			delete(h.byUser, userID)
		}
		h.byIP[clientIP]--
		if h.byIP[clientIP] == 0 {
			delete(h.byIP, clientIP)
		}
		h.mu.Unlock()
		logStreamActiveTotal.Add(-1)
		conn.close()
	}, true, ""
}

// PublishTaskLog sends a task log entry to all WebSocket subscribers watching
// that task. It never blocks; slow clients are disconnected.
func (h *logStreamHub) PublishTaskLog(taskID string, entry any) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	h.mu.Lock()
	conns := h.subs[taskID]
	// Copy slice so we can release the lock before sending.
	targets := make([]*logStreamConn, 0, len(conns))
	for c := range conns {
		targets = append(targets, c)
	}
	h.mu.Unlock()

	for _, c := range targets {
		select {
		case c.send <- data:
		default:
			// Slow client — close and remove.
			c.close()
			h.mu.Lock()
			delete(h.subs[taskID], c)
			h.mu.Unlock()
		}
	}
}

// ---------------------------------------------------------------------------
// IP extraction helper
// ---------------------------------------------------------------------------

// clientIP extracts the best-effort client IP from a request.  It honours the
// X-Forwarded-For header (first hop) as set by a trusted reverse proxy, and
// falls back to RemoteAddr when the header is absent.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For: client, proxy1, proxy2 — take the first entry.
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Strip port from RemoteAddr (host:port or [host]:port).
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		addr = addr[:idx]
	}
	return strings.Trim(addr, "[]")
}

// ---------------------------------------------------------------------------
// HTTP handler
// ---------------------------------------------------------------------------

// handleStreamTaskLogs upgrades an HTTP connection to a WebSocket and streams
// log entries for the given task ID in real-time.
//
// GET /api/v1/tasks/{id}/logs/stream
//
// Query parameters:
//   - after_id: only send logs with ID > after_id (for resuming a stream)
//
// Each message is a JSON-encoded db.TaskLog object.
//
// Connection caps are enforced before the WebSocket upgrade:
//   - 429 Too Many Requests when per-user or per-IP limit is exceeded.
func (s *Server) handleStreamTaskLogs(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing task id")
		return
	}

	// Identify the caller from the auth context (already set by Middleware).
	claims, _ := auth.FromContext(r.Context())
	userID := ""
	if claims != nil {
		userID = claims.UserID
	}
	ip := clientIP(r)

	// Enforce connection caps before the upgrade so we can send HTTP 429.
	_, ok, reason := s.logHub.checkCap(userID, ip)
	if !ok {
		switch reason {
		case "user_limit":
			logStreamRejectedUser.Add(1)
		case "ip_limit":
			logStreamRejectedIP.Add(1)
		}
		s.logger.Warn("log stream connection rejected", "reason", reason, "user_id", userID, "ip", ip)
		w.Header().Set("Retry-After", "60")
		writeProblem(w, r, http.StatusTooManyRequests, "Too Many Requests",
			"log-stream connection limit exceeded ("+reason+")")
		return
	}

	var afterID int64
	if raw := r.URL.Query().Get("after_id"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			afterID = n
		}
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("log stream ws upgrade", "err", err)
		return
	}

	lsConn := &logStreamConn{
		conn: conn,
		send: make(chan []byte, 256),
	}

	// Register with the per-task hub so live pushes are received.
	cleanup, ok, reason := s.logHub.subscribe(taskID, userID, ip, lsConn)
	if !ok {
		// Cap was hit in the narrow window between checkCap and subscribe
		// (another goroutine got in first).  The upgrade already happened so
		// we can only close gracefully.
		switch reason {
		case "user_limit":
			logStreamRejectedUser.Add(1)
		case "ip_limit":
			logStreamRejectedIP.Add(1)
		}
		s.logger.Warn("log stream connection rejected after upgrade", "reason", reason, "user_id", userID, "ip", ip)
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "connection limit exceeded"))
		conn.Close()
		return
	}
	defer cleanup()

	// Send historical logs first (backfill up to the point the client joined).
	go func() {
		logs, err := s.store.TailTaskLogs(r.Context(), taskID, afterID)
		if err != nil {
			s.logger.Warn("log stream backfill", "err", err, "task_id", taskID)
			return
		}
		for _, lg := range logs {
			data, err := json.Marshal(lg)
			if err == nil {
				select {
				case lsConn.send <- data:
				default:
				}
			}
		}
	}()

	go logStreamWritePump(lsConn, s.logger)
	logStreamReadPump(lsConn)
}

// checkCap is a read-only probe that returns false when the connection would
// exceed a cap, without modifying any counters.  It is used to reject before
// upgrading the HTTP connection.
func (h *logStreamHub) checkCap(userID, clientIP string) (_ struct{}, ok bool, reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cfg.MaxPerUser > 0 && h.byUser[userID] >= h.cfg.MaxPerUser {
		return struct{}{}, false, "user_limit"
	}
	if h.cfg.MaxPerIP > 0 && h.byIP[clientIP] >= h.cfg.MaxPerIP {
		return struct{}{}, false, "ip_limit"
	}
	return struct{}{}, true, ""
}

// ---------------------------------------------------------------------------
// Write and read pumps (unchanged from original)
// ---------------------------------------------------------------------------

// logStreamWritePump writes messages from the send channel to the WebSocket.
func logStreamWritePump(c *logStreamConn, logger *slog.Logger) {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		c.close()
	}()

	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// logStreamReadPump reads from the connection to handle pongs / disconnects.
func logStreamReadPump(c *logStreamConn) {
	defer c.close()

	c.conn.SetReadLimit(wsMaxMsgSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}
