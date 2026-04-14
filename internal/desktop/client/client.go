package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

// Envelope matches the server's JSON response envelope.
type Envelope struct {
	Data json.RawMessage `json:"data"`
	Meta map[string]any  `json:"meta"`
}

// Problem is an RFC 9457 error response.
type Problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail"`
	Instance  string `json:"instance"`
	RequestID string `json:"request_id"`
}

func (p *Problem) Error() string {
	if p.Detail != "" {
		return fmt.Sprintf("%s: %s", p.Title, p.Detail)
	}
	return p.Title
}

// Collection holds paginated results.
type Collection[T any] struct {
	Items      []T
	TotalCount int64
	NextCursor string
}

// Client is the HTTP client for the EncodeSwarmr REST API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// New creates a new Client targeting the given base URL.
func New(baseURL string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Jar: jar,
		},
	}
}

// SetAPIKey configures an API key to be sent on every request.
func (c *Client) SetAPIKey(key string) {
	c.apiKey = key
}

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// request performs an HTTP request and decodes the response envelope into result.
func (c *Client) request(ctx context.Context, method, path string, body any, result any) error {
	u := c.baseURL + "/api/v1" + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var p Problem
		if json.Unmarshal(respBody, &p) == nil && p.Title != "" {
			return &p
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if result == nil {
		return nil
	}

	var env Envelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	return json.Unmarshal(env.Data, result)
}

// requestCollection decodes a paginated collection response.
func requestCollection[T any](c *Client, ctx context.Context, path string) (*Collection[T], error) {
	u := c.baseURL + "/api/v1" + path

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var p Problem
		if json.Unmarshal(respBody, &p) == nil && p.Title != "" {
			return nil, &p
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var env Envelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}

	var items []T
	if err := json.Unmarshal(env.Data, &items); err != nil {
		return nil, err
	}

	col := &Collection[T]{Items: items}
	if tc, ok := env.Meta["total_count"].(float64); ok {
		col.TotalCount = int64(tc)
	}
	if nc, ok := env.Meta["next_cursor"].(string); ok {
		col.NextCursor = nc
	}
	return col, nil
}

// buildQuery builds a URL query string from params, skipping empty values.
func buildQuery(params map[string]string) string {
	v := url.Values{}
	for k, val := range params {
		if val != "" {
			v.Set(k, val)
		}
	}
	s := v.Encode()
	if s != "" {
		return "?" + s
	}
	return ""
}

// requestRaw makes a raw HTTP request outside the /api/v1 prefix (e.g. auth endpoints).
func (c *Client) requestRaw(ctx context.Context, method, path string, body any) (*http.Response, error) {
	u := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	return c.httpClient.Do(req)
}
