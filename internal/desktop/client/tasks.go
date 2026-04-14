package client

import "context"

// GetTask returns the details of a single task by ID.
func (c *Client) GetTask(ctx context.Context, id string) (*Task, error) {
	var task Task
	if err := c.request(ctx, "GET", "/tasks/"+id, nil, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// ListTaskLogs returns all log entries for a task, ordered by timestamp.
func (c *Client) ListTaskLogs(ctx context.Context, taskID string) ([]LogEntry, error) {
	var logs []LogEntry
	if err := c.request(ctx, "GET", "/tasks/"+taskID+"/logs", nil, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}
