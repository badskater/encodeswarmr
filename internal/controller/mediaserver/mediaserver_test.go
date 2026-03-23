package mediaserver

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew_EmptyConfig_ReturnsManagerWithNoServers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := New(nil, logger)
	if m == nil {
		t.Fatal("New(nil) returned nil Manager")
	}
	if len(m.Servers()) != 0 {
		t.Errorf("Servers() len = %d, want 0", len(m.Servers()))
	}
}

func TestNew_CreatesServerForEachKnownType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfgs := []config.MediaServerConfig{
		{Name: "My Plex", Type: "plex", URL: "http://plex.local:32400", Token: "abc"},
		{Name: "My Jellyfin", Type: "jellyfin", URL: "http://jf.local:8096", APIKey: "key1"},
		{Name: "My Emby", Type: "emby", URL: "http://emby.local:8096", APIKey: "key2"},
	}
	m := New(cfgs, logger)
	if len(m.Servers()) != 3 {
		t.Errorf("Servers() len = %d, want 3", len(m.Servers()))
	}
}

func TestNew_SkipsUnknownType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfgs := []config.MediaServerConfig{
		{Name: "Plex", Type: "plex", URL: "http://plex.local:32400"},
		{Name: "Kodi", Type: "kodi", URL: "http://kodi.local"},  // unknown
	}
	m := New(cfgs, logger)
	// only plex should be registered
	if len(m.Servers()) != 1 {
		t.Errorf("Servers() len = %d, want 1 (kodi skipped)", len(m.Servers()))
	}
}

func TestNew_ServerNamesAndTypes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfgs := []config.MediaServerConfig{
		{Name: "PlexServer", Type: "plex"},
		{Name: "JellyServer", Type: "jellyfin"},
		{Name: "EmbyServer", Type: "emby"},
	}
	m := New(cfgs, logger)
	byName := map[string]MediaServer{}
	for _, s := range m.Servers() {
		byName[s.Name()] = s
	}

	if s, ok := byName["PlexServer"]; !ok {
		t.Error("PlexServer not found")
	} else if s.Type() != "plex" {
		t.Errorf("PlexServer type = %q, want plex", s.Type())
	}

	if s, ok := byName["JellyServer"]; !ok {
		t.Error("JellyServer not found")
	} else if s.Type() != "jellyfin" {
		t.Errorf("JellyServer type = %q, want jellyfin", s.Type())
	}

	if s, ok := byName["EmbyServer"]; !ok {
		t.Error("EmbyServer not found")
	} else if s.Type() != "emby" {
		t.Errorf("EmbyServer type = %q, want emby", s.Type())
	}
}

// ---------------------------------------------------------------------------
// GetByName
// ---------------------------------------------------------------------------

func TestGetByName_Found(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := New([]config.MediaServerConfig{
		{Name: "Plex", Type: "plex", URL: "http://plex.local"},
	}, logger)
	s, err := m.GetByName("Plex")
	if err != nil {
		t.Fatalf("GetByName(Plex): %v", err)
	}
	if s.Name() != "Plex" {
		t.Errorf("Name() = %q, want Plex", s.Name())
	}
}

func TestGetByName_NotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := New(nil, logger)
	_, err := m.GetByName("missing")
	if err == nil {
		t.Error("expected error for unknown name")
	}
}

// ---------------------------------------------------------------------------
// Plex RefreshLibrary
// ---------------------------------------------------------------------------

func TestPlex_RefreshLibrary_CallsCorrectURL(t *testing.T) {
	var capturedPath string
	var capturedToken string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedToken = r.Header.Get("X-Plex-Token")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := newPlexClient(config.MediaServerConfig{
		Name:  "Plex",
		URL:   ts.URL,
		Token: "my-plex-token",
	})
	if err := c.RefreshLibrary(context.Background()); err != nil {
		t.Fatalf("RefreshLibrary: %v", err)
	}

	if capturedPath != "/library/sections/all/refresh" {
		t.Errorf("path = %q, want /library/sections/all/refresh", capturedPath)
	}
	if capturedToken != "my-plex-token" {
		t.Errorf("X-Plex-Token = %q, want my-plex-token", capturedToken)
	}
}

func TestPlex_RefreshLibrary_WithLibraryID(t *testing.T) {
	var capturedPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := newPlexClient(config.MediaServerConfig{
		Name:      "Plex",
		URL:       ts.URL,
		Token:     "tok",
		LibraryID: "42",
	})
	if err := c.RefreshLibrary(context.Background()); err != nil {
		t.Fatalf("RefreshLibrary: %v", err)
	}

	if capturedPath != "/library/sections/42/refresh" {
		t.Errorf("path = %q, want /library/sections/42/refresh", capturedPath)
	}
}

func TestPlex_RefreshLibrary_HTTP4xx_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := newPlexClient(config.MediaServerConfig{Name: "Plex", URL: ts.URL})
	if err := c.RefreshLibrary(context.Background()); err == nil {
		t.Error("expected error for 401 response")
	}
}

func TestPlex_RefreshLibrary_ConnectionRefused_ReturnsError(t *testing.T) {
	c := newPlexClient(config.MediaServerConfig{
		Name: "Plex",
		URL:  "http://127.0.0.1:1", // no server
	})
	if err := c.RefreshLibrary(context.Background()); err == nil {
		t.Error("expected error for unreachable host")
	}
}

func TestPlex_NotifyNewContent_CallsRefresh(t *testing.T) {
	called := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := newPlexClient(config.MediaServerConfig{Name: "Plex", URL: ts.URL})
	if err := c.NotifyNewContent(context.Background(), "/some/path.mkv"); err != nil {
		t.Fatalf("NotifyNewContent: %v", err)
	}
	if called != 1 {
		t.Errorf("server called %d times, want 1", called)
	}
}

// ---------------------------------------------------------------------------
// Jellyfin RefreshLibrary
// ---------------------------------------------------------------------------

func TestJellyfin_RefreshLibrary_CallsCorrectURL(t *testing.T) {
	var capturedPath string
	var capturedToken string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedToken = r.Header.Get("X-Emby-Token")
		w.WriteHeader(http.StatusNoContent) // Jellyfin returns 204
	}))
	defer ts.Close()

	c := newJellyfinClient(config.MediaServerConfig{
		Name:   "Jellyfin",
		URL:    ts.URL,
		APIKey: "jf-api-key",
	})
	if err := c.RefreshLibrary(context.Background()); err != nil {
		t.Fatalf("RefreshLibrary: %v", err)
	}

	if capturedPath != "/Library/Refresh" {
		t.Errorf("path = %q, want /Library/Refresh", capturedPath)
	}
	if capturedToken != "jf-api-key" {
		t.Errorf("X-Emby-Token = %q, want jf-api-key", capturedToken)
	}
}

func TestJellyfin_RefreshLibrary_UsesPostMethod(t *testing.T) {
	var capturedMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newJellyfinClient(config.MediaServerConfig{Name: "Jellyfin", URL: ts.URL})
	_ = c.RefreshLibrary(context.Background())

	if capturedMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", capturedMethod)
	}
}

func TestJellyfin_RefreshLibrary_HTTP4xx_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	c := newJellyfinClient(config.MediaServerConfig{Name: "Jellyfin", URL: ts.URL})
	if err := c.RefreshLibrary(context.Background()); err == nil {
		t.Error("expected error for 403 response")
	}
}

// ---------------------------------------------------------------------------
// Emby RefreshLibrary
// ---------------------------------------------------------------------------

func TestEmby_RefreshLibrary_CallsCorrectURL(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newEmbyClient(config.MediaServerConfig{
		Name:   "Emby",
		URL:    ts.URL,
		APIKey: "emby-secret",
	})
	if err := c.RefreshLibrary(context.Background()); err != nil {
		t.Fatalf("RefreshLibrary: %v", err)
	}

	if !strings.Contains(capturedURL, "/Library/Refresh") {
		t.Errorf("URL %q does not contain /Library/Refresh", capturedURL)
	}
	if !strings.Contains(capturedURL, "api_key=emby-secret") {
		t.Errorf("URL %q does not contain api_key", capturedURL)
	}
}

func TestEmby_RefreshLibrary_UsesPostMethod(t *testing.T) {
	var capturedMethod string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newEmbyClient(config.MediaServerConfig{Name: "Emby", URL: ts.URL})
	_ = c.RefreshLibrary(context.Background())

	if capturedMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", capturedMethod)
	}
}

func TestEmby_RefreshLibrary_HTTP4xx_ReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := newEmbyClient(config.MediaServerConfig{Name: "Emby", URL: ts.URL})
	if err := c.RefreshLibrary(context.Background()); err == nil {
		t.Error("expected error for 401 response")
	}
}

// ---------------------------------------------------------------------------
// TriggerAutoRefresh
// ---------------------------------------------------------------------------

func TestTriggerAutoRefresh_OnlyRefreshesNamedServers(t *testing.T) {
	var plexCalled, jellyfinCalled int
	tsA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plexCalled++
		w.WriteHeader(http.StatusOK)
	}))
	defer tsA.Close()

	tsB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jellyfinCalled++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer tsB.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := New([]config.MediaServerConfig{
		{Name: "PlexA", Type: "plex", URL: tsA.URL},
		{Name: "JellyB", Type: "jellyfin", URL: tsB.URL},
	}, logger)

	// Only refresh PlexA.
	m.TriggerAutoRefresh(context.Background(), []string{"PlexA"})
	time.Sleep(200 * time.Millisecond)

	if plexCalled != 1 {
		t.Errorf("plexCalled = %d, want 1", plexCalled)
	}
	if jellyfinCalled != 0 {
		t.Errorf("jellyfinCalled = %d, want 0 (not in auto-refresh list)", jellyfinCalled)
	}
}

func TestTriggerAutoRefresh_EmptyNames_RefreshesAll(t *testing.T) {
	var plexCalled, jellyfinCalled int
	tsA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plexCalled++
		w.WriteHeader(http.StatusOK)
	}))
	defer tsA.Close()

	tsB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jellyfinCalled++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer tsB.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := New([]config.MediaServerConfig{
		{Name: "PlexA", Type: "plex", URL: tsA.URL},
		{Name: "JellyB", Type: "jellyfin", URL: tsB.URL},
	}, logger)

	// Empty names = refresh all.
	m.TriggerAutoRefresh(context.Background(), []string{})
	time.Sleep(200 * time.Millisecond)

	if plexCalled != 1 {
		t.Errorf("plexCalled = %d, want 1", plexCalled)
	}
	if jellyfinCalled != 1 {
		t.Errorf("jellyfinCalled = %d, want 1", jellyfinCalled)
	}
}

func TestTriggerAutoRefresh_NoServers_NoError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := New(nil, logger)
	// Should not panic.
	m.TriggerAutoRefresh(context.Background(), []string{"SomeServer"})
	time.Sleep(50 * time.Millisecond)
}
