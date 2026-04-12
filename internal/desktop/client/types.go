package client

import (
	"encoding/json"
	"time"
)

// Job statuses
const (
	JobQueued    = "queued"
	JobWaiting   = "waiting"
	JobAssigned  = "assigned"
	JobRunning   = "running"
	JobCompleted = "completed"
	JobFailed    = "failed"
	JobCancelled = "cancelled"
)

// Task statuses
const (
	TaskPending   = "pending"
	TaskAssigned  = "assigned"
	TaskRunning   = "running"
	TaskCompleted = "completed"
	TaskFailed    = "failed"
)

// Agent statuses
const (
	AgentIdle            = "idle"
	AgentRunning         = "running"
	AgentOffline         = "offline"
	AgentDraining        = "draining"
	AgentPendingApproval = "pending_approval"
)

// AudioConfig describes per-job audio encoding parameters.
type AudioConfig struct {
	Codec      string  `json:"codec"`
	Bitrate    *string `json:"bitrate,omitempty"`
	Channels   *int    `json:"channels,omitempty"`
	SampleRate *int    `json:"sample_rate,omitempty"`
}

// AudioPreset describes a built-in audio encoding configuration.
type AudioPreset struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Codec       string   `json:"codec"`
	Bitrate     *string  `json:"bitrate,omitempty"`
	Channels    *int     `json:"channels,omitempty"`
	SampleRate  *int     `json:"sample_rate,omitempty"`
	Params      string   `json:"params"`
	Tags        []string `json:"tags"`
}

// Job represents an encoding job.
type Job struct {
	ID             string       `json:"id"`
	SourceID       string       `json:"source_id"`
	SourcePath     string       `json:"source_path"`
	JobType        string       `json:"job_type"`
	Status         string       `json:"status"`
	Priority       int          `json:"priority"`
	TasksTotal     int          `json:"tasks_total"`
	TasksCompleted int          `json:"tasks_completed"`
	TasksFailed    int          `json:"tasks_failed"`
	TasksPending   int          `json:"tasks_pending"`
	TasksRunning   int          `json:"tasks_running"`
	DependsOn      *string      `json:"depends_on"`
	ChainGroup     *string      `json:"chain_group"`
	TargetTags     []string     `json:"target_tags"`
	AudioConfig    *AudioConfig `json:"audio_config"`
	ETASeconds     *float64     `json:"eta_seconds"`
	ETAHuman       *string      `json:"eta_human"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// JobDetail holds a job and its associated tasks.
type JobDetail struct {
	Job   Job    `json:"job"`
	Tasks []Task `json:"tasks"`
}

// Task represents a single encoding task within a job.
type Task struct {
	ID            string     `json:"id"`
	JobID         string     `json:"job_id"`
	AgentID       *string    `json:"agent_id"`
	Status        string     `json:"status"`
	ChunkIndex    int        `json:"chunk_index"`
	ExitCode      *int       `json:"exit_code"`
	ErrorMsg      *string    `json:"error_msg"`
	ErrorCategory *string    `json:"error_category,omitempty"`
	FramesEncoded *int64     `json:"frames_encoded"`
	AvgFPS        *float64   `json:"avg_fps"`
	OutputSize    *int64     `json:"output_size"`
	RetryCount    int        `json:"retry_count,omitempty"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Agent represents an encoding agent.
type Agent struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Hostname      string     `json:"hostname"`
	IPAddress     string     `json:"ip_address"`
	Status        string     `json:"status"`
	Tags          []string   `json:"tags"`
	AgentVersion  string     `json:"agent_version"`
	OSVersion     string     `json:"os_version"`
	CPUCount      int        `json:"cpu_count"`
	RAMMIB        int        `json:"ram_mib"`
	GPUVendor     *string    `json:"gpu_vendor"`
	GPUModel      *string    `json:"gpu_model"`
	GPUEnabled    bool       `json:"gpu_enabled"`
	NVENC         bool       `json:"nvenc"`
	QSV           bool       `json:"qsv"`
	AMF           bool       `json:"amf"`
	VNCPort       int        `json:"vnc_port"`
	LastHeartbeat *time.Time `json:"last_heartbeat"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdateChannel string     `json:"update_channel"`
}

// Source represents a media source file.
type Source struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Filename    string    `json:"filename"`
	SizeBytes   int64     `json:"size_bytes"`
	DurationSec *float64  `json:"duration_sec"`
	State       string    `json:"state"`
	VMafScore   *float64  `json:"vmaf_score"`
	CloudURI    *string   `json:"cloud_uri"`
	HDRType     string    `json:"hdr_type"`
	DVProfile   int       `json:"dv_profile"`
	Thumbnails  []string  `json:"thumbnails"`
	CreatedAt   time.Time `json:"created_at"`
}

// SubtitleTrack describes a single subtitle stream in a media file.
type SubtitleTrack struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Codec    string `json:"codec"`
	Title    string `json:"title"`
}

// FileEntry describes a file or directory in the file manager.
type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Ext     string    `json:"ext"`
	IsVideo bool      `json:"is_video"`
}

// FileInfo extends FileEntry with optional codec information.
type FileInfo struct {
	FileEntry
	CodecInfo json.RawMessage `json:"codec_info,omitempty"`
}

// Template represents a script template (avs, vpy, bat).
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Extension   string    `json:"extension"`
	Content     string    `json:"content"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Variable is a global script variable stored in the database.
type Variable struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Value       string    `json:"value"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Webhook represents a configured notification webhook.
type Webhook struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// WebhookDelivery is a single webhook delivery attempt.
type WebhookDelivery struct {
	ID           int        `json:"id"`
	WebhookID    string     `json:"webhook_id"`
	Event        string     `json:"event"`
	ResponseCode *int       `json:"response_code"`
	Success      bool       `json:"success"`
	Attempt      int        `json:"attempt"`
	ErrorMsg     *string    `json:"error_msg"`
	DeliveredAt  time.Time  `json:"delivered_at"`
}

// User represents a system user.
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// LogEntry is a single task log line.
type LogEntry struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Stream    string    `json:"stream"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// AnalysisFramePoint is a single frame data point from analysis.
type AnalysisFramePoint struct {
	Frame int      `json:"frame"`
	Score *float64 `json:"score,omitempty"`
	PTS   *float64 `json:"pts,omitempty"`
}

// AnalysisSummary holds aggregate analysis metrics.
type AnalysisSummary struct {
	Mean        *float64 `json:"mean,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	PSNR        *float64 `json:"psnr,omitempty"`
	SSIM        *float64 `json:"ssim,omitempty"`
	Width       *int     `json:"width,omitempty"`
	Height      *int     `json:"height,omitempty"`
	DurationSec *float64 `json:"duration_sec,omitempty"`
	FrameCount  *int64   `json:"frame_count,omitempty"`
	Codec       *string  `json:"codec,omitempty"`
	BitRate     *int64   `json:"bit_rate,omitempty"`
	SceneCount  *int     `json:"scene_count,omitempty"`
}

// AnalysisResult is the result of a source analysis pass.
type AnalysisResult struct {
	ID        string               `json:"id"`
	SourceID  string               `json:"source_id"`
	Type      string               `json:"type"`
	FrameData []AnalysisFramePoint `json:"frame_data,omitempty"`
	Summary   *AnalysisSummary     `json:"summary,omitempty"`
	CreatedAt time.Time            `json:"created_at"`
}

// PathMapping maps a Windows UNC prefix to a Linux path prefix.
type PathMapping struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	WindowsPrefix string    `json:"windows_prefix"`
	LinuxPrefix   string    `json:"linux_prefix"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// EnrollmentToken is a one-time agent enrollment token.
type EnrollmentToken struct {
	ID        string     `json:"id"`
	Token     string     `json:"token"`
	CreatedBy string     `json:"created_by"`
	UsedBy    *string    `json:"used_by"`
	UsedAt    *time.Time `json:"used_at"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// SceneBoundary is a single scene cut point.
type SceneBoundary struct {
	Frame    int     `json:"frame"`
	PTS      float64 `json:"pts"`
	Timecode string  `json:"timecode"`
}

// SceneData is the response from the scenes endpoint.
type SceneData struct {
	SourceID    string          `json:"source_id"`
	FPS         float64         `json:"fps"`
	TotalFrames int64           `json:"total_frames"`
	DurationSec float64         `json:"duration_sec"`
	Scenes      []SceneBoundary `json:"scenes"`
}

// ChunkingConfig holds scene-based auto-chunking parameters.
type ChunkingConfig struct {
	EnableChunking   bool `json:"enable_chunking"`
	ChunkSizeFrames  int  `json:"chunk_size_frames"`
	OverlapFrames    int  `json:"overlap_frames"`
}

// Schedule is a cron-based job scheduling rule.
type Schedule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	CronExpr    string          `json:"cron_expr"`
	JobTemplate json.RawMessage `json:"job_template"`
	Enabled     bool            `json:"enabled"`
	LastRunAt   *time.Time      `json:"last_run_at"`
	NextRunAt   *time.Time      `json:"next_run_at"`
	CreatedAt   time.Time       `json:"created_at"`
}

// ThroughputPoint is a single hourly throughput bucket.
type ThroughputPoint struct {
	Hour      string `json:"hour"`
	Completed int    `json:"completed"`
}

// QueueSummary summarises the current task queue depth.
type QueueSummary struct {
	Pending               int      `json:"pending"`
	Running               int      `json:"running"`
	EstimatedCompletionSec *float64 `json:"estimated_completion_sec"`
}

// ActivityEvent is an entry in the recent job activity feed.
type ActivityEvent struct {
	JobID      string    `json:"job_id"`
	SourcePath string    `json:"source_path"`
	Status     string    `json:"status"`
	ChangedAt  time.Time `json:"changed_at"`
}

// Plugin represents an installed encoding plugin.
type Plugin struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	Description string  `json:"description"`
	Enabled     bool    `json:"enabled"`
	Author      *string `json:"author"`
}

// NotificationPrefs holds per-user notification preferences.
type NotificationPrefs struct {
	ID                   string    `json:"id"`
	UserID               string    `json:"user_id"`
	NotifyOnJobComplete  bool      `json:"notify_on_job_complete"`
	NotifyOnJobFailed    bool      `json:"notify_on_job_failed"`
	NotifyOnAgentStale   bool      `json:"notify_on_agent_stale"`
	WebhookFilterUserOnly bool     `json:"webhook_filter_user_only"`
	EmailAddress         string    `json:"email_address"`
	NotifyEmail          bool      `json:"notify_email"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// AutoScalingSettings holds the auto-scaling configuration.
type AutoScalingSettings struct {
	Enabled             bool    `json:"enabled"`
	WebhookURL          string  `json:"webhook_url"`
	ScaleUpThreshold    float64 `json:"scale_up_threshold"`
	ScaleDownThreshold  float64 `json:"scale_down_threshold"`
	CooldownSeconds     int     `json:"cooldown_seconds"`
}

// AgentPool is a named tag group for organising agents.
type AgentPool struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Color       string    `json:"color"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// QueueStatus holds the current queue state.
type QueueStatus struct {
	Paused              bool   `json:"paused"`
	Pending             int    `json:"pending"`
	Running             int    `json:"running"`
	EstimatedCompletion string `json:"estimated_completion"`
}

// ComparisonSource holds per-file metrics for a job comparison.
type ComparisonSource struct {
	DurationSec float64 `json:"duration_sec"`
	FileSizeMB  float64 `json:"file_size_mb"`
	Codec       *string `json:"codec,omitempty"`
	Resolution  *string `json:"resolution,omitempty"`
}

// ComparisonResponse holds source vs output metrics for a completed job.
type ComparisonResponse struct {
	Source           ComparisonSource `json:"source"`
	Output           ComparisonSource `json:"output"`
	CompressionRatio float64          `json:"compression_ratio"`
	SizeReductionPct float64          `json:"size_reduction_pct"`
	VMafScore        *float64         `json:"vmaf_score,omitempty"`
	PSNR             *float64         `json:"psnr,omitempty"`
	SSIM             *float64         `json:"ssim,omitempty"`
}

// NodeCategory is the category of a flow node.
type NodeCategory string

const (
	NodeCategoryInput        NodeCategory = "input"
	NodeCategoryEncoding     NodeCategory = "encoding"
	NodeCategoryAnalysis     NodeCategory = "analysis"
	NodeCategoryCondition    NodeCategory = "condition"
	NodeCategoryAudio        NodeCategory = "audio"
	NodeCategorySubtitle     NodeCategory = "subtitle"
	NodeCategoryOutput       NodeCategory = "output"
	NodeCategoryNotification NodeCategory = "notification"
	NodeCategoryFlow         NodeCategory = "flow"
	NodeCategoryTemplate     NodeCategory = "template"
)

// FlowNodeData is the data payload attached to every flow node.
type FlowNodeData struct {
	Label       string          `json:"label"`
	Category    NodeCategory    `json:"category"`
	Description string          `json:"description,omitempty"`
	Config      json.RawMessage `json:"config"`
	NodeType    string          `json:"nodeType"`
	Icon        string          `json:"icon"`
}

// Flow is a visual encoding workflow stored in the database.
type Flow struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Nodes       json.RawMessage `json:"nodes"`
	Edges       json.RawMessage `json:"edges"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// NodeTemplate is a palette entry for building flows.
type NodeTemplate struct {
	Type          string          `json:"type"`
	Label         string          `json:"label"`
	Category      NodeCategory    `json:"category"`
	Description   string          `json:"description"`
	Icon          string          `json:"icon"`
	DefaultConfig json.RawMessage `json:"defaultConfig"`
	Inputs        int             `json:"inputs"`
	Outputs       int             `json:"outputs"`
}

// WatchFolder represents a monitored directory for auto-import.
type WatchFolder struct {
	Name               string     `json:"name"`
	Path               string     `json:"path"`
	WindowsPath        string     `json:"windows_path"`
	FilePatterns       []string   `json:"file_patterns"`
	PollInterval       string     `json:"poll_interval"`
	AutoAnalyze        bool       `json:"auto_analyze"`
	MoveAfterAnalysis  *string    `json:"move_after_analysis,omitempty"`
	Enabled            bool       `json:"enabled"`
	LastScan           *time.Time `json:"last_scan,omitempty"`
}

// RuleCondition is a single condition in an encoding rule.
type RuleCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// RuleAction is the suggested action when rule conditions match.
type RuleAction struct {
	SuggestTemplateID  *string  `json:"suggest_template_id,omitempty"`
	SuggestAudioCodec  *string  `json:"suggest_audio_codec,omitempty"`
	SuggestPriority    *int     `json:"suggest_priority,omitempty"`
	SuggestTags        []string `json:"suggest_tags,omitempty"`
}

// EncodingRule is a rule that suggests encoding parameters based on source metadata.
type EncodingRule struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Priority   int             `json:"priority"`
	Conditions []RuleCondition `json:"conditions"`
	Actions    RuleAction      `json:"actions"`
	Enabled    bool            `json:"enabled"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// EncodingProfile is a named encoding configuration preset.
type EncodingProfile struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Container   string          `json:"container"`
	Settings    json.RawMessage `json:"settings"`
	AudioConfig json.RawMessage `json:"audio_config,omitempty"`
	CreatedBy   string          `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// AgentMetric is a single agent resource usage sample.
type AgentMetric struct {
	ID         int       `json:"id"`
	AgentID    string    `json:"agent_id"`
	CPUPct     float64   `json:"cpu_pct"`
	GPUPct     float64   `json:"gpu_pct"`
	MemPct     float64   `json:"mem_pct"`
	RecordedAt time.Time `json:"recorded_at"`
}

// AgentEncodingStats holds aggregate encoding stats for an agent.
type AgentEncodingStats struct {
	AgentID        string  `json:"agent_id"`
	TotalTasks     int     `json:"total_tasks"`
	CompletedTasks int     `json:"completed_tasks"`
	FailedTasks    int     `json:"failed_tasks"`
	AvgFPS         float64 `json:"avg_fps"`
	TotalFrames    int64   `json:"total_frames"`
}

// AgentHealthResponse is the deep-dive agent health response.
type AgentHealthResponse struct {
	Agent         Agent              `json:"agent"`
	EncodingStats AgentEncodingStats `json:"encoding_stats"`
}

// RecentTask is a summary of a recently completed task on an agent.
type RecentTask struct {
	ID            string     `json:"id"`
	JobID         string     `json:"job_id"`
	ChunkIndex    int        `json:"chunk_index"`
	TaskType      string     `json:"task_type"`
	Status        string     `json:"status"`
	AvgFPS        *float64   `json:"avg_fps"`
	FramesEncoded *int64     `json:"frames_encoded"`
	ErrorMsg      *string    `json:"error_msg"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// UpgradeChannel represents an agent software update channel.
type UpgradeChannel struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AuditEntry is a single audit log record.
type AuditEntry struct {
	ID         int        `json:"id"`
	UserID     *string    `json:"user_id"`
	Username   string     `json:"username"`
	Action     string     `json:"action"`
	Resource   string     `json:"resource"`
	ResourceID string     `json:"resource_id"`
	IPAddress  string     `json:"ip_address"`
	LoggedAt   time.Time  `json:"logged_at"`
}

// UserSession represents an active user session.
type UserSession struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// APIKeyInfo holds metadata for an API key.
type APIKeyInfo struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	RateLimit  int        `json:"rate_limit"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

// EncodeConfig is the per-job encoding configuration block.
type EncodeConfig struct {
	RunScriptTemplateID    *string            `json:"run_script_template_id,omitempty"`
	FrameserverTemplateID  *string            `json:"frameserver_template_id,omitempty"`
	ChunkBoundaries        []ChunkBoundary    `json:"chunk_boundaries,omitempty"`
	OutputRoot             *string            `json:"output_root,omitempty"`
	OutputExtension        *string            `json:"output_extension,omitempty"`
	ExtraVars              map[string]string  `json:"extra_vars,omitempty"`
	ChunkingConfig         *ChunkingConfig    `json:"chunking_config,omitempty"`
}

// ChunkBoundary describes the frame range for a single encoding chunk.
type ChunkBoundary struct {
	StartFrame int `json:"start_frame"`
	EndFrame   int `json:"end_frame"`
}

// CreateJobRequest is the request body for creating a job.
type CreateJobRequest struct {
	SourceID    string        `json:"source_id"`
	JobType     string        `json:"job_type"`
	Priority    *int          `json:"priority,omitempty"`
	TargetTags  []string      `json:"target_tags,omitempty"`
	EncodeConfig *EncodeConfig `json:"encode_config,omitempty"`
	AudioConfig *AudioConfig  `json:"audio_config,omitempty"`
	DependsOn   *string       `json:"depends_on,omitempty"`
	ChainGroup  *string       `json:"chain_group,omitempty"`
	FlowID      *string       `json:"flow_id,omitempty"`
}

// ChainStep is a single step in a job chain.
type ChainStep struct {
	JobType     string        `json:"job_type"`
	Name        *string       `json:"name,omitempty"`
	Priority    *int          `json:"priority,omitempty"`
	TargetTags  []string      `json:"target_tags,omitempty"`
	EncodeConfig *EncodeConfig `json:"encode_config,omitempty"`
	AudioConfig *AudioConfig  `json:"audio_config,omitempty"`
}

// CreateJobChainRequest is the request body for creating a job chain.
type CreateJobChainRequest struct {
	SourceID string      `json:"source_id"`
	Steps    []ChainStep `json:"steps"`
}

// JobChainResponse is the response from the job chain endpoints.
type JobChainResponse struct {
	ChainGroup string `json:"chain_group"`
	Jobs       []Job  `json:"jobs"`
}

// BatchImportRequest is the request body for batch source import.
type BatchImportRequest struct {
	PathPattern      string  `json:"path_pattern"`
	Recursive        bool    `json:"recursive,omitempty"`
	AutoAnalyze      bool    `json:"auto_analyze,omitempty"`
	AutoEncode       bool    `json:"auto_encode,omitempty"`
	EncodeTemplateID *string `json:"encode_template_id,omitempty"`
}

// BatchImportResult describes the outcome for a single file in a batch import.
type BatchImportResult struct {
	Path       string  `json:"path"`
	SourceID   *string `json:"source_id,omitempty"`
	JobID      *string `json:"job_id,omitempty"`
	Skipped    bool    `json:"skipped,omitempty"`
	SkipReason *string `json:"skip_reason,omitempty"`
	Error      *string `json:"error,omitempty"`
}

// BatchImportResponse is the response from the batch import endpoint.
type BatchImportResponse struct {
	Imported int                 `json:"imported"`
	Results  []BatchImportResult `json:"results"`
}

// EvaluateRulesRequest is the request body for rule evaluation.
type EvaluateRulesRequest struct {
	SourceID    *string  `json:"source_id,omitempty"`
	Resolution  *string  `json:"resolution,omitempty"`
	HDRType     *string  `json:"hdr_type,omitempty"`
	Codec       *string  `json:"codec,omitempty"`
	FileSizeGB  *float64 `json:"file_size_gb,omitempty"`
	DurationMin *float64 `json:"duration_min,omitempty"`
}

// EvaluateRulesResponse is the response from the rule evaluation endpoint.
type EvaluateRulesResponse struct {
	Matched    bool        `json:"matched"`
	Suggestion *RuleAction `json:"suggestion"`
}
