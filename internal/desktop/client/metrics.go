package client

import (
	"context"
	"strconv"
)

// GetThroughput returns hourly completed-task counts for the given number of hours.
func (c *Client) GetThroughput(ctx context.Context, hours int) ([]ThroughputPoint, error) {
	q := buildQuery(map[string]string{"hours": strconv.Itoa(hours)})
	var points []ThroughputPoint
	if err := c.request(ctx, "GET", "/metrics/throughput"+q, nil, &points); err != nil {
		return nil, err
	}
	return points, nil
}

// GetQueueSummary returns the current task queue depth and estimated completion.
func (c *Client) GetQueueSummary(ctx context.Context) (*QueueSummary, error) {
	var summary QueueSummary
	if err := c.request(ctx, "GET", "/metrics/queue", nil, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

// GetRecentActivity returns the most recent job status change events.
func (c *Client) GetRecentActivity(ctx context.Context, limit int) ([]ActivityEvent, error) {
	q := buildQuery(map[string]string{"limit": strconv.Itoa(limit)})
	var events []ActivityEvent
	if err := c.request(ctx, "GET", "/metrics/activity"+q, nil, &events); err != nil {
		return nil, err
	}
	return events, nil
}
