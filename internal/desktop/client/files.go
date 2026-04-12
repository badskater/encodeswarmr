package client

import (
	"context"
	"fmt"
)

// BrowseFilesResponse is the response from the file browse endpoint.
type BrowseFilesResponse struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

// BrowseFiles lists files and directories under the given path.
func (c *Client) BrowseFiles(ctx context.Context, path string) (*BrowseFilesResponse, error) {
	q := buildQuery(map[string]string{"path": path})
	var resp BrowseFilesResponse
	if err := c.request(ctx, "GET", "/files/browse"+q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFileInfo returns metadata and codec information for a single file.
func (c *Client) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	q := buildQuery(map[string]string{"path": path})
	var info FileInfo
	if err := c.request(ctx, "GET", "/files/info"+q, nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// MoveFileResponse is the response from the file move endpoint.
type MoveFileResponse struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Moved       bool   `json:"moved"`
}

// MoveFile moves a file from source to destination on the server.
func (c *Client) MoveFile(ctx context.Context, source, destination string) (*MoveFileResponse, error) {
	body := map[string]string{"source": source, "destination": destination}
	var resp MoveFileResponse
	if err := c.request(ctx, "POST", "/files/move", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteFile removes a file on the server.
func (c *Client) DeleteFile(ctx context.Context, path string) error {
	q := buildQuery(map[string]string{"path": path})
	return c.request(ctx, "DELETE", "/files/delete"+q, nil, nil)
}

// FileDownloadURL returns a URL that triggers a file download from the server.
func (c *Client) FileDownloadURL(path string) string {
	q := buildQuery(map[string]string{"path": path})
	return fmt.Sprintf("%s/api/v1/files/download%s", c.baseURL, q)
}
