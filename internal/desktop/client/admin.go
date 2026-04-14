package client

import (
	"context"
	"net/url"
	"strconv"
)

// --- Templates ---

// ListTemplates returns all script templates.
func (c *Client) ListTemplates(ctx context.Context) ([]Template, error) {
	var templates []Template
	if err := c.request(ctx, "GET", "/templates", nil, &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

// GetTemplate returns a single template by ID.
func (c *Client) GetTemplate(ctx context.Context, id string) (*Template, error) {
	var t Template
	if err := c.request(ctx, "GET", "/templates/"+id, nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateTemplate creates a new script template.
func (c *Client) CreateTemplate(ctx context.Context, body map[string]any) (*Template, error) {
	var t Template
	if err := c.request(ctx, "POST", "/templates", body, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateTemplate replaces a template's content.
func (c *Client) UpdateTemplate(ctx context.Context, id string, body map[string]any) (*Template, error) {
	var t Template
	if err := c.request(ctx, "PUT", "/templates/"+id, body, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// DeleteTemplate removes a script template.
func (c *Client) DeleteTemplate(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/templates/"+id, nil, nil)
}

// TemplatePreviewResponse is the response from the template preview endpoint.
type TemplatePreviewResponse struct {
	TemplateName string `json:"template_name"`
	Extension    string `json:"extension"`
	Content      string `json:"content"`
}

// PreviewTemplate renders a template with optional source ID and variable overrides.
func (c *Client) PreviewTemplate(ctx context.Context, id, sourceID string, variables map[string]string) (*TemplatePreviewResponse, error) {
	body := map[string]any{
		"source_id": sourceID,
		"variables": variables,
	}
	var resp TemplatePreviewResponse
	if err := c.request(ctx, "POST", "/templates/"+id+"/preview", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Variables ---

// ListVariables returns all global script variables.
func (c *Client) ListVariables(ctx context.Context) ([]Variable, error) {
	var vars []Variable
	if err := c.request(ctx, "GET", "/variables", nil, &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

// UpsertVariable creates or updates a global variable by name.
func (c *Client) UpsertVariable(ctx context.Context, name, value, description string) (*Variable, error) {
	body := map[string]string{"value": value, "description": description}
	var v Variable
	if err := c.request(ctx, "PUT", "/variables/"+name, body, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// DeleteVariable removes a global variable by ID.
func (c *Client) DeleteVariable(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/variables/"+id, nil, nil)
}

// --- Webhooks ---

// ListWebhooks returns all configured webhooks.
func (c *Client) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	var hooks []Webhook
	if err := c.request(ctx, "GET", "/webhooks", nil, &hooks); err != nil {
		return nil, err
	}
	return hooks, nil
}

// CreateWebhook creates a new webhook.
func (c *Client) CreateWebhook(ctx context.Context, body map[string]any) (*Webhook, error) {
	var hook Webhook
	if err := c.request(ctx, "POST", "/webhooks", body, &hook); err != nil {
		return nil, err
	}
	return &hook, nil
}

// UpdateWebhook replaces a webhook's configuration.
func (c *Client) UpdateWebhook(ctx context.Context, id string, body map[string]any) (*Webhook, error) {
	var hook Webhook
	if err := c.request(ctx, "PUT", "/webhooks/"+id, body, &hook); err != nil {
		return nil, err
	}
	return &hook, nil
}

// DeleteWebhook removes a webhook.
func (c *Client) DeleteWebhook(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/webhooks/"+id, nil, nil)
}

// TestWebhook fires a test delivery to a webhook.
func (c *Client) TestWebhook(ctx context.Context, id string) error {
	return c.request(ctx, "POST", "/webhooks/"+id+"/test", nil, nil)
}

// ListWebhookDeliveries returns delivery history for a webhook.
func (c *Client) ListWebhookDeliveries(ctx context.Context, id string, limit, offset int) ([]WebhookDelivery, error) {
	q := buildQuery(map[string]string{
		"limit":  strconv.Itoa(limit),
		"offset": strconv.Itoa(offset),
	})
	var deliveries []WebhookDelivery
	if err := c.request(ctx, "GET", "/webhooks/"+id+"/deliveries"+q, nil, &deliveries); err != nil {
		return nil, err
	}
	return deliveries, nil
}

// --- Users ---

// ListUsers returns all users.
func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	var users []User
	if err := c.request(ctx, "GET", "/users", nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// CreateUser creates a new user account.
func (c *Client) CreateUser(ctx context.Context, username, email, role, password string) (*User, error) {
	body := map[string]string{
		"username": username,
		"email":    email,
		"role":     role,
		"password": password,
	}
	var u User
	if err := c.request(ctx, "POST", "/users", body, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// DeleteUser removes a user account.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/users/"+id, nil, nil)
}

// UpdateUserRole changes the role of a user.
func (c *Client) UpdateUserRole(ctx context.Context, id, role string) error {
	body := map[string]string{"role": role}
	return c.request(ctx, "PUT", "/users/"+id+"/role", body, nil)
}

// --- Path Mappings ---

// ListPathMappings returns all UNC-to-Linux path mappings.
func (c *Client) ListPathMappings(ctx context.Context) ([]PathMapping, error) {
	var mappings []PathMapping
	if err := c.request(ctx, "GET", "/path-mappings", nil, &mappings); err != nil {
		return nil, err
	}
	return mappings, nil
}

// CreatePathMapping creates a new path mapping.
func (c *Client) CreatePathMapping(ctx context.Context, name, windowsPrefix, linuxPrefix string, enabled bool) (*PathMapping, error) {
	body := map[string]any{
		"name":           name,
		"windows_prefix": windowsPrefix,
		"linux_prefix":   linuxPrefix,
		"enabled":        enabled,
	}
	var pm PathMapping
	if err := c.request(ctx, "POST", "/path-mappings", body, &pm); err != nil {
		return nil, err
	}
	return &pm, nil
}

// UpdatePathMapping replaces a path mapping's configuration.
func (c *Client) UpdatePathMapping(ctx context.Context, id string, body map[string]any) (*PathMapping, error) {
	var pm PathMapping
	if err := c.request(ctx, "PUT", "/path-mappings/"+id, body, &pm); err != nil {
		return nil, err
	}
	return &pm, nil
}

// DeletePathMapping removes a path mapping.
func (c *Client) DeletePathMapping(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/path-mappings/"+id, nil, nil)
}

// --- Enrollment Tokens ---

// ListEnrollmentTokens returns all agent enrollment tokens.
func (c *Client) ListEnrollmentTokens(ctx context.Context) ([]EnrollmentToken, error) {
	var tokens []EnrollmentToken
	if err := c.request(ctx, "GET", "/agent-tokens", nil, &tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}

// CreateEnrollmentToken creates a new one-time agent enrollment token.
// expiresAt is an optional ISO 8601 timestamp; pass empty string for no expiry.
func (c *Client) CreateEnrollmentToken(ctx context.Context, expiresAt string) (*EnrollmentToken, error) {
	body := map[string]string{}
	if expiresAt != "" {
		body["expires_at"] = expiresAt
	}
	var token EnrollmentToken
	if err := c.request(ctx, "POST", "/agent-tokens", body, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

// DeleteEnrollmentToken removes an enrollment token.
func (c *Client) DeleteEnrollmentToken(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/agent-tokens/"+id, nil, nil)
}

// --- Schedules ---

// ListSchedules returns all job schedules.
func (c *Client) ListSchedules(ctx context.Context) ([]Schedule, error) {
	var schedules []Schedule
	if err := c.request(ctx, "GET", "/schedules", nil, &schedules); err != nil {
		return nil, err
	}
	return schedules, nil
}

// GetSchedule returns a single schedule by ID.
func (c *Client) GetSchedule(ctx context.Context, id string) (*Schedule, error) {
	var s Schedule
	if err := c.request(ctx, "GET", "/schedules/"+id, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateSchedule creates a new cron-based job schedule.
func (c *Client) CreateSchedule(ctx context.Context, body map[string]any) (*Schedule, error) {
	var s Schedule
	if err := c.request(ctx, "POST", "/schedules", body, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// UpdateSchedule replaces a schedule's configuration.
func (c *Client) UpdateSchedule(ctx context.Context, id string, body map[string]any) (*Schedule, error) {
	var s Schedule
	if err := c.request(ctx, "PUT", "/schedules/"+id, body, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteSchedule removes a schedule.
func (c *Client) DeleteSchedule(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/schedules/"+id, nil, nil)
}

// --- Plugins ---

// ListPlugins returns all installed encoding plugins.
func (c *Client) ListPlugins(ctx context.Context) ([]Plugin, error) {
	var plugins []Plugin
	if err := c.request(ctx, "GET", "/plugins", nil, &plugins); err != nil {
		return nil, err
	}
	return plugins, nil
}

// TogglePlugin enables or disables a plugin by name.
func (c *Client) TogglePlugin(ctx context.Context, name string, enable bool) (*Plugin, error) {
	action := "disable"
	if enable {
		action = "enable"
	}
	var p Plugin
	if err := c.request(ctx, "PUT", "/plugins/"+name+"/"+action, nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// --- Flows ---

// ListFlows returns all visual encoding workflows.
func (c *Client) ListFlows(ctx context.Context) ([]Flow, error) {
	var flows []Flow
	if err := c.request(ctx, "GET", "/flows", nil, &flows); err != nil {
		return nil, err
	}
	return flows, nil
}

// GetFlow returns a single flow by ID.
func (c *Client) GetFlow(ctx context.Context, id string) (*Flow, error) {
	var f Flow
	if err := c.request(ctx, "GET", "/flows/"+id, nil, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// CreateFlow creates a new flow.
func (c *Client) CreateFlow(ctx context.Context, body map[string]any) (*Flow, error) {
	var f Flow
	if err := c.request(ctx, "POST", "/flows", body, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// UpdateFlow replaces a flow's node/edge graph.
func (c *Client) UpdateFlow(ctx context.Context, id string, body map[string]any) (*Flow, error) {
	var f Flow
	if err := c.request(ctx, "PUT", "/flows/"+id, body, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// DeleteFlow removes a flow.
func (c *Client) DeleteFlow(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/flows/"+id, nil, nil)
}

// --- Audio Presets ---

// ListAudioPresets returns all built-in audio encoding presets.
func (c *Client) ListAudioPresets(ctx context.Context) ([]AudioPreset, error) {
	var presets []AudioPreset
	if err := c.request(ctx, "GET", "/presets/audio", nil, &presets); err != nil {
		return nil, err
	}
	return presets, nil
}

// --- Watch Folders ---

// ListWatchFolders returns all configured watch folder entries.
func (c *Client) ListWatchFolders(ctx context.Context) ([]WatchFolder, error) {
	var folders []WatchFolder
	if err := c.request(ctx, "GET", "/watch-folders", nil, &folders); err != nil {
		return nil, err
	}
	return folders, nil
}

// ToggleWatchFolder enables or disables a watch folder by name.
func (c *Client) ToggleWatchFolder(ctx context.Context, name string, enabled bool) error {
	action := "disable"
	if enabled {
		action = "enable"
	}
	return c.request(ctx, "PUT", "/watch-folders/"+url.PathEscape(name)+"/"+action, nil, nil)
}

// ScanWatchFolder triggers an immediate scan of a watch folder.
func (c *Client) ScanWatchFolder(ctx context.Context, name string) error {
	return c.request(ctx, "POST", "/watch-folders/"+url.PathEscape(name)+"/scan", nil, nil)
}

// --- Encoding Rules ---

// ListEncodingRules returns all encoding rules.
func (c *Client) ListEncodingRules(ctx context.Context) ([]EncodingRule, error) {
	var rules []EncodingRule
	if err := c.request(ctx, "GET", "/encoding-rules", nil, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// GetEncodingRule returns a single encoding rule by ID.
func (c *Client) GetEncodingRule(ctx context.Context, id string) (*EncodingRule, error) {
	var rule EncodingRule
	if err := c.request(ctx, "GET", "/encoding-rules/"+id, nil, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// CreateEncodingRule creates a new encoding rule.
func (c *Client) CreateEncodingRule(ctx context.Context, body map[string]any) (*EncodingRule, error) {
	var rule EncodingRule
	if err := c.request(ctx, "POST", "/encoding-rules", body, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// UpdateEncodingRule replaces an encoding rule's configuration.
func (c *Client) UpdateEncodingRule(ctx context.Context, id string, body map[string]any) (*EncodingRule, error) {
	var rule EncodingRule
	if err := c.request(ctx, "PUT", "/encoding-rules/"+id, body, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// DeleteEncodingRule removes an encoding rule.
func (c *Client) DeleteEncodingRule(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/encoding-rules/"+id, nil, nil)
}

// EvaluateEncodingRules evaluates rules against hypothetical source metadata.
func (c *Client) EvaluateEncodingRules(ctx context.Context, req *EvaluateRulesRequest) (*EvaluateRulesResponse, error) {
	var resp EvaluateRulesResponse
	if err := c.request(ctx, "POST", "/encoding-rules/evaluate", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// --- Encoding Profiles ---

// ListEncodingProfiles returns all encoding profiles.
func (c *Client) ListEncodingProfiles(ctx context.Context) ([]EncodingProfile, error) {
	var profiles []EncodingProfile
	if err := c.request(ctx, "GET", "/encoding-profiles", nil, &profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

// GetEncodingProfile returns a single encoding profile by ID.
func (c *Client) GetEncodingProfile(ctx context.Context, id string) (*EncodingProfile, error) {
	var profile EncodingProfile
	if err := c.request(ctx, "GET", "/encoding-profiles/"+id, nil, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// CreateEncodingProfile creates a new encoding profile.
func (c *Client) CreateEncodingProfile(ctx context.Context, body map[string]any) (*EncodingProfile, error) {
	var profile EncodingProfile
	if err := c.request(ctx, "POST", "/encoding-profiles", body, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// UpdateEncodingProfile replaces an encoding profile.
func (c *Client) UpdateEncodingProfile(ctx context.Context, id string, body map[string]any) (*EncodingProfile, error) {
	var profile EncodingProfile
	if err := c.request(ctx, "PUT", "/encoding-profiles/"+id, body, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// DeleteEncodingProfile removes an encoding profile.
func (c *Client) DeleteEncodingProfile(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/encoding-profiles/"+id, nil, nil)
}

// --- Agent Pools ---

// ListAgentPools returns all agent pools.
func (c *Client) ListAgentPools(ctx context.Context) ([]AgentPool, error) {
	var pools []AgentPool
	if err := c.request(ctx, "GET", "/agent-pools", nil, &pools); err != nil {
		return nil, err
	}
	return pools, nil
}

// CreateAgentPool creates a new agent pool.
func (c *Client) CreateAgentPool(ctx context.Context, name, description, color string, tags []string) (*AgentPool, error) {
	body := map[string]any{
		"name":        name,
		"description": description,
		"color":       color,
		"tags":        tags,
	}
	var pool AgentPool
	if err := c.request(ctx, "POST", "/agent-pools", body, &pool); err != nil {
		return nil, err
	}
	return &pool, nil
}

// UpdateAgentPool replaces an agent pool's configuration.
func (c *Client) UpdateAgentPool(ctx context.Context, id, name, description, color string, tags []string) (*AgentPool, error) {
	body := map[string]any{
		"name":        name,
		"description": description,
		"color":       color,
		"tags":        tags,
	}
	var pool AgentPool
	if err := c.request(ctx, "PUT", "/agent-pools/"+id, body, &pool); err != nil {
		return nil, err
	}
	return &pool, nil
}

// DeleteAgentPool removes an agent pool.
func (c *Client) DeleteAgentPool(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/agent-pools/"+id, nil, nil)
}

// --- Audit Log ---

// ListAuditLog returns a paginated audit log.
func (c *Client) ListAuditLog(ctx context.Context, limit, offset int) (*Collection[AuditEntry], error) {
	q := buildQuery(map[string]string{
		"limit":  strconv.Itoa(limit),
		"offset": strconv.Itoa(offset),
	})
	return requestCollection[AuditEntry](c, ctx, "/audit-log"+q)
}

// GetUserActivity returns the audit log for a specific user.
func (c *Client) GetUserActivity(ctx context.Context, userID string, limit, offset int) (*Collection[AuditEntry], error) {
	q := buildQuery(map[string]string{
		"limit":  strconv.Itoa(limit),
		"offset": strconv.Itoa(offset),
	})
	return requestCollection[AuditEntry](c, ctx, "/users/"+userID+"/activity"+q)
}

// AuditLogExportURL returns the URL for downloading an audit log export.
func (c *Client) AuditLogExportURL(format string, limit int) string {
	q := buildQuery(map[string]string{
		"format": format,
		"limit":  strconv.Itoa(limit),
	})
	return c.baseURL + "/api/v1/audit-logs/export" + q
}

// --- Notification Preferences ---

// GetNotificationPrefs returns the current user's notification preferences.
func (c *Client) GetNotificationPrefs(ctx context.Context) (*NotificationPrefs, error) {
	var prefs NotificationPrefs
	if err := c.request(ctx, "GET", "/me/notifications", nil, &prefs); err != nil {
		return nil, err
	}
	return &prefs, nil
}

// UpdateNotificationPrefs updates the current user's notification preferences.
func (c *Client) UpdateNotificationPrefs(ctx context.Context, body map[string]any) (*NotificationPrefs, error) {
	var prefs NotificationPrefs
	if err := c.request(ctx, "PUT", "/me/notifications", body, &prefs); err != nil {
		return nil, err
	}
	return &prefs, nil
}

// --- Auto-Scaling ---

// GetAutoScaling returns the auto-scaling settings.
func (c *Client) GetAutoScaling(ctx context.Context) (*AutoScalingSettings, error) {
	var settings AutoScalingSettings
	if err := c.request(ctx, "GET", "/settings/auto-scaling", nil, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

// UpdateAutoScaling replaces the auto-scaling settings.
func (c *Client) UpdateAutoScaling(ctx context.Context, body map[string]any) (*AutoScalingSettings, error) {
	var settings AutoScalingSettings
	if err := c.request(ctx, "PUT", "/settings/auto-scaling", body, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

// TestAutoScalingWebhook fires a test call to the auto-scaling webhook.
func (c *Client) TestAutoScalingWebhook(ctx context.Context) error {
	return c.request(ctx, "POST", "/settings/auto-scaling/test", nil, nil)
}

// --- Sessions ---

// ListSessions returns all active user sessions.
func (c *Client) ListSessions(ctx context.Context) ([]UserSession, error) {
	var sessions []UserSession
	if err := c.request(ctx, "GET", "/sessions", nil, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// DeleteSession revokes a user session by token/ID.
func (c *Client) DeleteSession(ctx context.Context, id string) error {
	return c.request(ctx, "DELETE", "/sessions/"+id, nil, nil)
}

// --- API Keys ---

// ListAPIKeys returns all API keys for the current user.
func (c *Client) ListAPIKeys(ctx context.Context) ([]APIKeyInfo, error) {
	var keys []APIKeyInfo
	if err := c.request(ctx, "GET", "/api-keys", nil, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// UpdateAPIKeyRateLimit sets the per-minute rate limit for an API key.
func (c *Client) UpdateAPIKeyRateLimit(ctx context.Context, id string, rateLimit int) error {
	body := map[string]int{"rate_limit": rateLimit}
	return c.request(ctx, "PUT", "/api-keys/"+id+"/rate-limit", body, nil)
}

// --- Notification Channel Tests ---

// TestEmail sends a test email notification.
func (c *Client) TestEmail(ctx context.Context, to string) error {
	body := map[string]string{"to": to}
	return c.request(ctx, "POST", "/notifications/test-email", body, nil)
}

// TestTelegram sends a test Telegram notification.
func (c *Client) TestTelegram(ctx context.Context) error {
	return c.request(ctx, "POST", "/notifications/test-telegram", map[string]any{}, nil)
}

// TestPushover sends a test Pushover notification.
func (c *Client) TestPushover(ctx context.Context) error {
	return c.request(ctx, "POST", "/notifications/test-pushover", map[string]any{}, nil)
}

// TestNtfy sends a test ntfy notification.
func (c *Client) TestNtfy(ctx context.Context) error {
	return c.request(ctx, "POST", "/notifications/test-ntfy", map[string]any{}, nil)
}
