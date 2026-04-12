package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// HubEvent matches the server's broadcast event envelope.
type HubEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// WSClient manages a WebSocket connection for live events from the controller.
type WSClient struct {
	conn      *websocket.Conn
	events    chan HubEvent
	done      chan struct{}
	closeOnce sync.Once
	logger    *slog.Logger
}

// ConnectWS establishes a WebSocket connection to the controller's event hub.
// The returned WSClient must be closed with Close() when no longer needed.
func (c *Client) ConnectWS(ctx context.Context, logger *slog.Logger) (*WSClient, error) {
	// Convert the HTTP base URL to a WebSocket URL.
	wsURL := strings.Replace(c.baseURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL += "/api/v1/ws"

	header := http.Header{}
	if c.apiKey != "" {
		header.Set("X-API-Key", c.apiKey)
	}

	// Forward session cookies so cookie-authenticated users are recognised.
	if c.httpClient.Jar != nil {
		u, err := url.Parse(c.baseURL)
		if err == nil {
			for _, cookie := range c.httpClient.Jar.Cookies(u) {
				header.Add("Cookie", cookie.String())
			}
		}
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("ws connect: %w", err)
	}

	ws := &WSClient{
		conn:   conn,
		events: make(chan HubEvent, 256),
		done:   make(chan struct{}),
		logger: logger,
	}
	go ws.readLoop()
	return ws, nil
}

// Events returns the channel on which incoming hub events are delivered.
// The channel is closed when the connection is terminated.
func (ws *WSClient) Events() <-chan HubEvent { return ws.events }

// Close shuts down the WebSocket connection gracefully.
func (ws *WSClient) Close() {
	ws.closeOnce.Do(func() {
		close(ws.done)
		ws.conn.Close()
	})
}

// readLoop reads messages from the WebSocket connection and forwards them to
// the events channel. It exits when the connection is closed or an error occurs.
func (ws *WSClient) readLoop() {
	defer close(ws.events)

	ws.conn.SetPongHandler(func(string) error {
		return ws.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	if err := ws.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		ws.logger.Warn("ws set read deadline", "err", err)
	}

	for {
		_, data, err := ws.conn.ReadMessage()
		if err != nil {
			select {
			case <-ws.done:
				// Normal close initiated by the caller.
			default:
				ws.logger.Error("ws read error", "err", err)
			}
			return
		}

		var evt HubEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			ws.logger.Warn("ws unmarshal failed", "err", err)
			continue
		}

		select {
		case ws.events <- evt:
		default:
			ws.logger.Warn("ws event dropped: consumer too slow")
		}
	}
}
