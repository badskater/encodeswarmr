package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// logStreamHub manages per-task WebSocket subscriptions for task log streaming.
// When new task log entries arrive (via gRPC StreamLogs), PublishTaskLog
// broadcasts them to every subscriber watching that task ID.
type logStreamHub struct {
	mu   sync.Mutex
	subs map[string]map[*logStreamConn]struct{} // taskID → set of connections
}

type logStreamConn struct {
	conn      *websocket.Conn
	send      chan []byte
	closeOnce sync.Once
}

func (c *logStreamConn) close() {
	c.closeOnce.Do(func() { c.conn.Close() })
}

// newLogStreamHub creates an idle hub.  No goroutine is needed — sends are
// direct (non-blocking) from PublishTaskLog.
func newLogStreamHub() *logStreamHub {
	return &logStreamHub{
		subs: make(map[string]map[*logStreamConn]struct{}),
	}
}

// subscribe registers conn as a subscriber for taskID and returns a cleanup fn.
func (h *logStreamHub) subscribe(taskID string, conn *logStreamConn) func() {
	h.mu.Lock()
	if h.subs[taskID] == nil {
		h.subs[taskID] = make(map[*logStreamConn]struct{})
	}
	h.subs[taskID][conn] = struct{}{}
	h.mu.Unlock()

	return func() {
		h.mu.Lock()
		delete(h.subs[taskID], conn)
		if len(h.subs[taskID]) == 0 {
			delete(h.subs, taskID)
		}
		h.mu.Unlock()
		conn.close()
	}
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

// handleStreamTaskLogs upgrades an HTTP connection to a WebSocket and streams
// log entries for the given task ID in real-time.
//
// GET /api/v1/tasks/{id}/logs/stream
//
// Query parameters:
//   - after_id: only send logs with ID > after_id (for resuming a stream)
//
// Each message is a JSON-encoded db.TaskLog object.
func (s *Server) handleStreamTaskLogs(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		writeProblem(w, r, http.StatusBadRequest, "Bad Request", "missing task id")
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
	cleanup := s.logHub.subscribe(taskID, lsConn)
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
