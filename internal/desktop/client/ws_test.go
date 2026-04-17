package client

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// newWSTestServer starts an httptest.Server whose single handler upgrades the
// connection and then calls serverFn with the server-side *websocket.Conn.
// The caller owns closing the server.
func newWSTestServer(t *testing.T, serverFn func(conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("ws upgrade error: %v", err)
			return
		}
		defer conn.Close()
		serverFn(conn)
	}))
	return srv
}

// wsURL converts an httptest.Server URL (http://…) to ws://…/api/v1/ws so
// that ConnectWS's URL transformation lands on the test server.
func wsURL(srv *httptest.Server) string {
	return strings.Replace(srv.URL, "http://", "ws://", 1) + "/api/v1/ws"
}

// httpBaseURL returns the plain http:// base URL of the test server so we can
// build a Client pointing at it.
func httpBaseURL(srv *httptest.Server) string {
	return srv.URL
}

// discardLogger returns a no-op slog.Logger suitable for tests.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(noopWriter{}, nil))
}

type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }

// hubEventJSON serialises a HubEvent to JSON bytes.
func hubEventJSON(t *testing.T, typ string, payload any) []byte {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	evt := HubEvent{Type: typ, Payload: json.RawMessage(raw)}
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal HubEvent: %v", err)
	}
	return b
}

// ---------------------------------------------------------------------------
// TestConnectWS_SuccessConnectReceiveClose
// Successful connect → receive one message → close cleanly
// ---------------------------------------------------------------------------

func TestConnectWS_SuccessConnectReceiveClose(t *testing.T) {
	srv := newWSTestServer(t, func(conn *websocket.Conn) {
		// Send one event then wait for the client to close.
		msg := hubEventJSON(t, "job.updated", map[string]string{"id": "j1"})
		_ = conn.WriteMessage(websocket.TextMessage, msg)
		// Hold the connection open until the client closes it.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	c := New(httpBaseURL(srv))
	ws, err := c.ConnectWS(context.Background(), discardLogger())
	if err != nil {
		t.Fatalf("ConnectWS() error = %v", err)
	}

	// Receive the event.
	select {
	case evt, ok := <-ws.Events():
		if !ok {
			t.Fatal("events channel closed unexpectedly before receiving event")
		}
		if evt.Type != "job.updated" {
			t.Errorf("Type = %q, want %q", evt.Type, "job.updated")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	// Close the client side.
	ws.Close()

	// After Close, the events channel must eventually be closed.
	select {
	case _, ok := <-ws.Events():
		if ok {
			// Drain any remaining events — channel may buffer the one we already read.
		}
		// ok==false means the channel was closed as expected.
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for events channel to close after ws.Close()")
	}
}

// ---------------------------------------------------------------------------
// TestConnectWS_ConnectFailure
// Dialing a refused / unreachable address returns an error.
// ---------------------------------------------------------------------------

func TestConnectWS_ConnectFailure(t *testing.T) {
	// Point at an address where nothing is listening.
	c := New("http://127.0.0.1:1") // port 1 is reserved, always refused
	_, err := c.ConnectWS(context.Background(), discardLogger())
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
	if !strings.Contains(err.Error(), "ws connect") {
		t.Errorf("error = %q, want it to contain 'ws connect'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// TestConnectWS_ContextCancellation
// Cancelling the context before the dial completes surfaces an error.
// ---------------------------------------------------------------------------

func TestConnectWS_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	c := New("http://127.0.0.1:1")
	_, err := c.connectWSWithDialer(ctx, discardLogger(), websocket.DefaultDialer)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestConnectWS_MessageRouting
// Multiple events of different types are all forwarded to Events().
// ---------------------------------------------------------------------------

func TestConnectWS_MessageRouting(t *testing.T) {
	types := []string{"job.created", "job.updated", "job.completed"}

	srv := newWSTestServer(t, func(conn *websocket.Conn) {
		for _, typ := range types {
			msg := hubEventJSON(t, typ, map[string]string{"t": typ})
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
		// Keep the connection alive until client closes.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	c := New(httpBaseURL(srv))
	ws, err := c.ConnectWS(context.Background(), discardLogger())
	if err != nil {
		t.Fatalf("ConnectWS() error = %v", err)
	}
	defer ws.Close()

	for _, want := range types {
		select {
		case evt, ok := <-ws.Events():
			if !ok {
				t.Fatalf("events channel closed prematurely, expected type %q", want)
			}
			if evt.Type != want {
				t.Errorf("Type = %q, want %q", evt.Type, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for event type %q", want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestConnectWS_InvalidJSONDropped
// A malformed JSON message is silently dropped; subsequent valid messages
// still arrive.
// ---------------------------------------------------------------------------

func TestConnectWS_InvalidJSONDropped(t *testing.T) {
	srv := newWSTestServer(t, func(conn *websocket.Conn) {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("not-json"))
		valid := hubEventJSON(t, "ping", nil)
		_ = conn.WriteMessage(websocket.TextMessage, valid)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	c := New(httpBaseURL(srv))
	ws, err := c.ConnectWS(context.Background(), discardLogger())
	if err != nil {
		t.Fatalf("ConnectWS() error = %v", err)
	}
	defer ws.Close()

	select {
	case evt, ok := <-ws.Events():
		if !ok {
			t.Fatal("events channel closed before receiving valid event")
		}
		if evt.Type != "ping" {
			t.Errorf("Type = %q, want %q", evt.Type, "ping")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for valid event after bad JSON")
	}
}

// ---------------------------------------------------------------------------
// TestConnectWS_GracefulShutdown
// Calling Close() is idempotent and the events channel closes.
// ---------------------------------------------------------------------------

func TestConnectWS_GracefulShutdown(t *testing.T) {
	srv := newWSTestServer(t, func(conn *websocket.Conn) {
		// Just keep the connection open until the client closes.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	c := New(httpBaseURL(srv))
	ws, err := c.ConnectWS(context.Background(), discardLogger())
	if err != nil {
		t.Fatalf("ConnectWS() error = %v", err)
	}

	// Call Close twice — the sync.Once must make this safe.
	ws.Close()
	ws.Close() // must not panic

	// Events channel must close.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-ws.Events():
			if !ok {
				return // channel closed — test passes
			}
		case <-deadline:
			t.Fatal("events channel did not close after ws.Close()")
		}
	}
}

// ---------------------------------------------------------------------------
// TestConnectWS_SlowConsumerDropsEvent
// When Events() channel is full, events are dropped rather than blocking the
// read loop (default: warn log, continue).
// ---------------------------------------------------------------------------

func TestConnectWS_SlowConsumerDropsEvent(t *testing.T) {
	// Send more messages than the channel buffer (256) would block on if the
	// read loop blocked. We only send a handful here and verify that the loop
	// itself does not stall.
	const msgCount = 300

	srv := newWSTestServer(t, func(conn *websocket.Conn) {
		for i := 0; i < msgCount; i++ {
			msg := hubEventJSON(t, "flood", map[string]int{"i": i})
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	c := New(httpBaseURL(srv))
	ws, err := c.ConnectWS(context.Background(), discardLogger())
	if err != nil {
		t.Fatalf("ConnectWS() error = %v", err)
	}

	// Do NOT consume from Events(). The readLoop must complete without blocking.
	// Give it time to finish writing all messages.
	time.Sleep(200 * time.Millisecond)
	ws.Close()

	// Drain the channel to confirm it closes cleanly.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-ws.Events():
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("events channel did not close after slow-consumer test")
		}
	}
}

// ---------------------------------------------------------------------------
// TestConnectWS_ServerCloseSignalsChannel
// When the server closes the connection, the Events() channel is closed so
// callers can detect EOF without polling.
// ---------------------------------------------------------------------------

func TestConnectWS_ServerCloseSignalsChannel(t *testing.T) {
	srv := newWSTestServer(t, func(conn *websocket.Conn) {
		// Send one message then close immediately.
		msg := hubEventJSON(t, "closing", nil)
		_ = conn.WriteMessage(websocket.TextMessage, msg)
		// Graceful WS close.
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
	})
	defer srv.Close()

	c := New(httpBaseURL(srv))
	ws, err := c.ConnectWS(context.Background(), discardLogger())
	if err != nil {
		t.Fatalf("ConnectWS() error = %v", err)
	}
	defer ws.Close()

	// Drain until channel closes.
	received := 0
	deadline := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-ws.Events():
			if !ok {
				// Channel closed as expected.
				return
			}
			received++
		case <-deadline:
			t.Fatalf("events channel did not close after server close (received %d events)", received)
		}
	}
}

// ---------------------------------------------------------------------------
// TestConnectWS_APIKeyForwarded
// When an API key is set, the Upgrade request carries X-API-Key.
// ---------------------------------------------------------------------------

func TestConnectWS_APIKeyForwarded(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Just close immediately.
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}))
	defer srv.Close()

	c := New(httpBaseURL(srv))
	c.SetAPIKey("super-secret")
	ws, err := c.ConnectWS(context.Background(), discardLogger())
	if err != nil {
		t.Fatalf("ConnectWS() error = %v", err)
	}
	defer ws.Close()

	if gotKey != "super-secret" {
		t.Errorf("X-API-Key = %q, want %q", gotKey, "super-secret")
	}
}

// ---------------------------------------------------------------------------
// TestConnectWS_URLSchemeConversion
// http:// → ws:// and https:// → wss:// conversions are applied correctly.
// ---------------------------------------------------------------------------

func TestWSURLConversion(t *testing.T) {
	cases := []struct {
		baseURL string
		wantWS  string
	}{
		{"http://localhost:8080", "ws://localhost:8080/api/v1/ws"},
		{"https://example.com", "wss://example.com/api/v1/ws"},
	}

	// We test the transformation by observing the URL the dialer receives via
	// a fake dialer — no network required.
	for _, tc := range cases {
		tc := tc
		t.Run(tc.baseURL, func(t *testing.T) {
			var gotURL string
			fake := &captureDialer{err: context.Canceled} // return an error so ConnectWS exits
			fake.capturedURL = &gotURL

			c := New(tc.baseURL)
			_, _ = c.connectWSWithDialer(context.Background(), discardLogger(), fake)

			if gotURL != tc.wantWS {
				t.Errorf("dialer received URL %q, want %q", gotURL, tc.wantWS)
			}
		})
	}
}

// captureDialer records the URL passed to DialContext and returns a
// configurable error so no real network connection is attempted.
type captureDialer struct {
	capturedURL *string
	err         error
}

func (d *captureDialer) DialContext(_ context.Context, urlStr string, _ http.Header) (*websocket.Conn, *http.Response, error) {
	if d.capturedURL != nil {
		*d.capturedURL = urlStr
	}
	return nil, nil, d.err
}
