package mediaserver

import (
	"context"
	"fmt"
	"net/http"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// embyClient implements MediaServer for Emby.
type embyClient struct {
	name   string
	url    string
	apiKey string
	httpCl *http.Client
}

func newEmbyClient(cfg config.MediaServerConfig) *embyClient {
	return &embyClient{
		name:   cfg.Name,
		url:    cfg.URL,
		apiKey: cfg.APIKey,
		httpCl: &http.Client{},
	}
}

func (e *embyClient) Name() string { return e.name }
func (e *embyClient) Type() string { return "emby" }

// RefreshLibrary calls POST /Library/Refresh on the Emby API.
func (e *embyClient) RefreshLibrary(ctx context.Context) error {
	endpoint := fmt.Sprintf("%s/Library/Refresh?api_key=%s", e.url, e.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("emby: build request: %w", err)
	}

	resp, err := e.httpCl.Do(req)
	if err != nil {
		return fmt.Errorf("emby: refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("emby: refresh returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// NotifyNewContent falls back to a full library refresh.
func (e *embyClient) NotifyNewContent(ctx context.Context, _ string) error {
	return e.RefreshLibrary(ctx)
}
