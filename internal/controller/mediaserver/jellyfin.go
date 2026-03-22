package mediaserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// jellyfinClient implements MediaServer for Jellyfin.
type jellyfinClient struct {
	name   string
	url    string
	apiKey string
	httpCl *http.Client
}

func newJellyfinClient(cfg config.MediaServerConfig) *jellyfinClient {
	return &jellyfinClient{
		name:   cfg.Name,
		url:    cfg.URL,
		apiKey: cfg.APIKey,
		httpCl: &http.Client{},
	}
}

func (j *jellyfinClient) Name() string { return j.name }
func (j *jellyfinClient) Type() string { return "jellyfin" }

// RefreshLibrary calls POST /Library/Refresh on the Jellyfin API.
func (j *jellyfinClient) RefreshLibrary(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/Library/Refresh", j.url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("jellyfin: build request: %w", err)
	}
	req.Header.Set("X-Emby-Token", j.apiKey)

	resp, err := j.httpCl.Do(req)
	if err != nil {
		return fmt.Errorf("jellyfin: refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("jellyfin: refresh returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// NotifyNewContent falls back to a full library refresh.
func (j *jellyfinClient) NotifyNewContent(ctx context.Context, _ string) error {
	return j.RefreshLibrary(ctx)
}
