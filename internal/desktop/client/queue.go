package client

import "context"

// GetQueueStatus returns the current pause state and queue depth.
func (c *Client) GetQueueStatus(ctx context.Context) (*QueueStatus, error) {
	var status QueueStatus
	if err := c.request(ctx, "GET", "/queue/status", nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// PauseQueueResponse is the response from the queue pause/resume endpoints.
type PauseQueueResponse struct {
	OK     bool `json:"ok"`
	Paused bool `json:"paused"`
}

// PauseQueue halts dispatch of new tasks from the queue.
func (c *Client) PauseQueue(ctx context.Context) (*PauseQueueResponse, error) {
	var resp PauseQueueResponse
	if err := c.request(ctx, "POST", "/queue/pause", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ResumeQueue re-enables task dispatch after a pause.
func (c *Client) ResumeQueue(ctx context.Context) (*PauseQueueResponse, error) {
	var resp PauseQueueResponse
	if err := c.request(ctx, "POST", "/queue/resume", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
