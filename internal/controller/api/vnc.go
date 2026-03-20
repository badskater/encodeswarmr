package api

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/gorilla/websocket"
)

var vncUpgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	// Allow any origin — the endpoint is already protected by session auth.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// handleAgentVNCProxy upgrades the HTTP connection to WebSocket and then
// proxies raw bytes between the browser (noVNC client) and the agent's VNC
// TCP socket. The agent must have a non-zero vnc_port stored in the DB.
//
// GET /api/v1/agents/{id}/vnc
func (s *Server) handleAgentVNCProxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	agent, err := s.store.GetAgentByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeProblem(w, r, http.StatusNotFound, "Not Found", "agent not found")
		return
	}
	if err != nil {
		s.logger.Error("vnc proxy: get agent", "err", err, "agent_id", id)
		writeProblem(w, r, http.StatusInternalServerError, "Internal Server Error", "")
		return
	}

	if agent.VNCPort == 0 {
		writeProblem(w, r, http.StatusConflict, "VNC Not Available",
			"agent has no VNC port configured — run 'setup-vnc' on the agent first")
		return
	}

	target := net.JoinHostPort(agent.IPAddress, fmt.Sprintf("%d", agent.VNCPort))

	// Open TCP connection to the agent's VNC port before upgrading WebSocket
	// so we can return a clean HTTP error if the connection fails.
	vncConn, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		s.logger.Warn("vnc proxy: could not connect to agent VNC port",
			"agent_id", id, "target", target, "err", err)
		writeProblem(w, r, http.StatusBadGateway, "VNC Unreachable",
			fmt.Sprintf("could not connect to %s: %v", target, err))
		return
	}

	// Upgrade the browser connection to WebSocket.
	wsConn, err := vncUpgrader.Upgrade(w, r, nil)
	if err != nil {
		vncConn.Close()
		s.logger.Warn("vnc proxy: websocket upgrade failed", "err", err)
		return
	}

	s.logger.Info("vnc proxy: session started",
		"agent_id", id, "target", target)

	proxyVNC(wsConn, vncConn)

	s.logger.Info("vnc proxy: session ended", "agent_id", id)
}

// proxyVNC copies bytes bidirectionally between a WebSocket connection
// (browser noVNC client) and a raw TCP connection (VNC server on agent).
// It exits when either side closes.
func proxyVNC(ws *websocket.Conn, tcp net.Conn) {
	defer ws.Close()
	defer tcp.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// TCP → WebSocket: read raw VNC protocol bytes, send as binary WS frames.
	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, err := tcp.Read(buf)
			if n > 0 {
				if werr := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					// Non-EOF errors from TCP are not unusual (remote close).
				}
				return
			}
		}
	}()

	// WebSocket → TCP: receive binary frames, write raw bytes to VNC server.
	go func() {
		defer wg.Done()
		for {
			msgType, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			if msgType != websocket.BinaryMessage && msgType != websocket.TextMessage {
				continue
			}
			if _, err := tcp.Write(data); err != nil {
				return
			}
		}
	}()

	wg.Wait()
}

// vncViewerTpl is the noVNC viewer HTML page template. It is served at
// /novnc/{id} and loads the noVNC JavaScript library from the configured
// base URL, then connects to the WebSocket proxy endpoint for the agent.
var vncViewerTpl = template.Must(template.New("vnc-viewer").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Remote Desktop — {{.AgentName}}</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
html, body { width: 100%; height: 100%; background: #1a1a2e; overflow: hidden; }
#toolbar {
  display: flex; align-items: center; gap: 12px;
  padding: 6px 12px; background: #16213e; color: #e0e0e0;
  font-family: system-ui, sans-serif; font-size: 13px;
}
#toolbar strong { color: #7ec8e3; }
#status { color: #aaa; }
#status.connected { color: #6bc46d; }
#status.error { color: #e06c75; }
#disconnect {
  margin-left: auto; padding: 4px 12px;
  background: #c0392b; color: #fff; border: none;
  border-radius: 4px; cursor: pointer; font-size: 12px;
}
#disconnect:hover { background: #e74c3c; }
#screen { width: 100%; height: calc(100vh - 34px); }
</style>
</head>
<body>
<div id="toolbar">
  <strong>{{.AgentName}}</strong>
  <span>{{.AgentIP}}</span>
  <span id="status">Connecting…</span>
  <button id="disconnect">Disconnect</button>
</div>
<div id="screen"></div>
<script type="module">
import RFB from '{{.NoVNCBase}}/core/rfb.js';

const wsProto = location.protocol === 'https:' ? 'wss:' : 'ws:';
const wsURL   = wsProto + '//' + location.host + '/api/v1/agents/{{.AgentID}}/vnc';
const status  = document.getElementById('status');
const screen  = document.getElementById('screen');

let rfb;

function connect() {
  rfb = new RFB(screen, wsURL);
  rfb.scaleViewport = true;
  rfb.resizeSession = true;

  rfb.addEventListener('connect', () => {
    status.textContent = 'Connected';
    status.className = 'connected';
  });
  rfb.addEventListener('disconnect', e => {
    status.textContent = e.detail.clean ? 'Disconnected' : 'Connection lost';
    status.className = 'error';
  });
  rfb.addEventListener('credentialsrequired', () => {
    const pass = prompt('VNC Password:');
    if (pass != null) rfb.sendCredentials({ password: pass });
  });
}

document.getElementById('disconnect').addEventListener('click', () => {
  if (rfb) rfb.disconnect();
  window.close();
});

connect();
</script>
</body>
</html>`))

// handleNoVNCViewer serves the noVNC viewer HTML page for the given agent.
//
// GET /novnc/{id}
func (s *Server) handleNoVNCViewer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	agent, err := s.store.GetAgentByID(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}
	if err != nil {
		s.logger.Error("novnc viewer: get agent", "err", err, "agent_id", id)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	noVNCBase := s.cfg.VNC.NoVNCBaseURL
	if noVNCBase == "" {
		noVNCBase = "https://unpkg.com/@novnc/novnc@1.5.0"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// No-cache: the page is generated per-request with the agent ID embedded.
	w.Header().Set("Cache-Control", "no-store")

	if err := vncViewerTpl.Execute(w, map[string]string{
		"AgentID":   agent.ID,
		"AgentName": agent.Name,
		"AgentIP":   agent.IPAddress,
		"NoVNCBase": noVNCBase,
	}); err != nil {
		s.logger.Error("novnc viewer: template execute", "err", err)
	}
}
