package mediaserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// plexClient implements MediaServer for Plex Media Server.
type plexClient struct {
	name      string
	url       string
	token     string
	libraryID string
	httpCl    *http.Client
}

func newPlexClient(cfg config.MediaServerConfig) *plexClient {
	return &plexClient{
		name:      cfg.Name,
		url:       cfg.URL,
		token:     cfg.Token,
		libraryID: cfg.LibraryID,
		httpCl:    &http.Client{},
	}
}

func (p *plexClient) Name() string { return p.name }
func (p *plexClient) Type() string { return "plex" }

// RefreshLibrary calls POST /library/sections/{id}/refresh on the Plex API.
// If no LibraryID is configured it refreshes all sections via /library/sections/all/refresh.
func (p *plexClient) RefreshLibrary(ctx context.Context) error {
	var endpoint string
	if p.libraryID != "" {
		endpoint = fmt.Sprintf("%s/library/sections/%s/refresh", p.url, p.libraryID)
	} else {
		endpoint = fmt.Sprintf("%s/library/sections/all/refresh", p.url)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("plex: build request: %w", err)
	}
	req.Header.Set("X-Plex-Token", p.token)

	resp, err := p.httpCl.Do(req)
	if err != nil {
		return fmt.Errorf("plex: refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("plex: refresh returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// NotifyNewContent falls back to a full library refresh for Plex since the
// single-item scan API requires the Plex metadata agent path.
func (p *plexClient) NotifyNewContent(ctx context.Context, _ string) error {
	return p.RefreshLibrary(ctx)
}
