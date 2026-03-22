// Package mediaserver provides integrations with media server software
// (Plex, Jellyfin, Emby) for automatic library refresh after job completion.
package mediaserver

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// MediaServer is the interface all media server clients must implement.
type MediaServer interface {
	// Name returns the human-readable name of this media server instance.
	Name() string
	// Type returns the server type: "plex", "jellyfin", or "emby".
	Type() string
	// RefreshLibrary triggers a full library scan on the media server.
	RefreshLibrary(ctx context.Context) error
	// NotifyNewContent notifies the media server about a specific new/updated
	// file at path. Some implementations may fall back to a full refresh.
	NotifyNewContent(ctx context.Context, path string) error
}

// Manager holds all configured media server clients and dispatches
// refresh calls after job completion.
type Manager struct {
	servers []MediaServer
	logger  *slog.Logger
}

// New builds a Manager from the provided configuration.
// Unknown server types are skipped with a warning rather than causing a fatal
// error, so that misconfigured entries don't block startup.
func New(cfgs []config.MediaServerConfig, logger *slog.Logger) *Manager {
	servers := make([]MediaServer, 0, len(cfgs))
	for _, c := range cfgs {
		var s MediaServer
		switch c.Type {
		case "plex":
			s = newPlexClient(c)
		case "jellyfin":
			s = newJellyfinClient(c)
		case "emby":
			s = newEmbyClient(c)
		default:
			logger.Warn("mediaserver: unknown type, skipping",
				"name", c.Name, "type", c.Type)
			continue
		}
		servers = append(servers, s)
	}
	return &Manager{servers: servers, logger: logger}
}

// Servers returns all configured media server clients.
func (m *Manager) Servers() []MediaServer {
	return m.servers
}

// GetByName returns the media server with the given name, or an error if not found.
func (m *Manager) GetByName(name string) (MediaServer, error) {
	for _, s := range m.servers {
		if s.Name() == name {
			return s, nil
		}
	}
	return nil, fmt.Errorf("media server %q not found", name)
}

// TriggerAutoRefresh fires RefreshLibrary on every server that has
// AutoRefresh enabled. Calls are made in separate goroutines so they do not
// block the caller (job completion path).
func (m *Manager) TriggerAutoRefresh(ctx context.Context, autoRefreshNames []string) {
	set := make(map[string]bool, len(autoRefreshNames))
	for _, n := range autoRefreshNames {
		set[n] = true
	}

	for _, s := range m.servers {
		if len(autoRefreshNames) > 0 && !set[s.Name()] {
			continue
		}
		srv := s
		go func() {
			if err := srv.RefreshLibrary(ctx); err != nil {
				m.logger.Warn("mediaserver: auto-refresh failed",
					"name", srv.Name(), "err", err)
			} else {
				m.logger.Info("mediaserver: auto-refresh triggered",
					"name", srv.Name())
			}
		}()
	}
}

// StatusInfo holds the display information returned by the list API.
type StatusInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	AutoRefresh bool   `json:"auto_refresh"`
}
