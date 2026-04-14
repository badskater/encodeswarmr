package client

import (
	"context"
	"strconv"
)

// ListAgents returns all registered agents.
func (c *Client) ListAgents(ctx context.Context) ([]Agent, error) {
	var agents []Agent
	if err := c.request(ctx, "GET", "/agents", nil, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

// GetAgent returns a single agent by ID.
func (c *Client) GetAgent(ctx context.Context, id string) (*Agent, error) {
	var agent Agent
	if err := c.request(ctx, "GET", "/agents/"+id, nil, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// DrainAgent puts an agent into draining state so it finishes its current task
// then goes idle without accepting new work.
func (c *Client) DrainAgent(ctx context.Context, id string) error {
	return c.request(ctx, "POST", "/agents/"+id+"/drain", nil, nil)
}

// ApproveAgent approves a pending-approval agent for use.
func (c *Client) ApproveAgent(ctx context.Context, id string) error {
	return c.request(ctx, "POST", "/agents/"+id+"/approve", nil, nil)
}

// ListAgentMetrics returns resource usage samples for an agent.
// window accepts values like "1h", "6h", "24h".
func (c *Client) ListAgentMetrics(ctx context.Context, agentID, window string) ([]AgentMetric, error) {
	q := buildQuery(map[string]string{"window": window})
	var metrics []AgentMetric
	if err := c.request(ctx, "GET", "/agents/"+agentID+"/metrics"+q, nil, &metrics); err != nil {
		return nil, err
	}
	return metrics, nil
}

// GetAgentHealth returns a deep-dive health report for an agent including encoding stats.
func (c *Client) GetAgentHealth(ctx context.Context, id string) (*AgentHealthResponse, error) {
	var health AgentHealthResponse
	if err := c.request(ctx, "GET", "/agents/"+id+"/health", nil, &health); err != nil {
		return nil, err
	}
	return &health, nil
}

// ListAgentRecentTasks returns recently completed tasks for an agent.
func (c *Client) ListAgentRecentTasks(ctx context.Context, id string, limit int) ([]RecentTask, error) {
	q := buildQuery(map[string]string{"limit": strconv.Itoa(limit)})
	var tasks []RecentTask
	if err := c.request(ctx, "GET", "/agents/"+id+"/recent-tasks"+q, nil, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// ListUpgradeChannels returns the available agent software update channels.
func (c *Client) ListUpgradeChannels(ctx context.Context) ([]UpgradeChannel, error) {
	var channels []UpgradeChannel
	if err := c.request(ctx, "GET", "/upgrade-channels", nil, &channels); err != nil {
		return nil, err
	}
	return channels, nil
}

// UpdateAgentChannel sets the update channel for an agent.
func (c *Client) UpdateAgentChannel(ctx context.Context, agentID, channel string) error {
	body := map[string]string{"channel": channel}
	return c.request(ctx, "PUT", "/agents/"+agentID+"/channel", body, nil)
}

// AssignAgentToPool adds an agent to an agent pool.
func (c *Client) AssignAgentToPool(ctx context.Context, agentID, poolID string) error {
	body := map[string]string{"pool_id": poolID}
	return c.request(ctx, "POST", "/agents/"+agentID+"/pools", body, nil)
}

// RemoveAgentFromPool removes an agent from a specific pool.
func (c *Client) RemoveAgentFromPool(ctx context.Context, agentID, poolID string) error {
	return c.request(ctx, "DELETE", "/agents/"+agentID+"/pools/"+poolID, nil, nil)
}
