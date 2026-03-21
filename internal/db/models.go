package db

import (
	"encoding/json"
	"time"
)

// EncodeConfig holds the job-level configuration used to expand a queued job
// into individual tasks and to generate per-task script files.
type EncodeConfig struct {
	RunScriptTemplateID   string            `json:"run_script_template_id"`
	FrameserverTemplateID string            `json:"frameserver_template_id,omitempty"`
	ChunkBoundaries       []ChunkBoundary   `json:"chunk_boundaries"`
	OutputRoot            string            `json:"output_root"`
	OutputExtension       string            `json:"output_extension,omitempty"` // default "mkv"
	ExtraVars             map[string]string `json:"extra_vars,omitempty"`
	// ChunkingConfig carries optional scene-based auto-chunking parameters sent
	// from the job-creation UI.  When present, the engine may use it for future
	// automatic boundary generation; currently it is stored for reference.
	ChunkingConfig *ChunkingConfig `json:"chunking_config,omitempty"`
}

// ChunkBoundary defines the inclusive frame range for one encoding task.
type ChunkBoundary struct {
	StartFrame int `json:"start_frame"`
	EndFrame   int `json:"end_frame"`
}

// ChunkingConfig carries the scene-based chunking parameters set in the job
// creation UI.
type ChunkingConfig struct {
	EnableChunking  bool `json:"enable_chunking"`
	ChunkSizeFrames int  `json:"chunk_size_frames"`
	OverlapFrames   int  `json:"overlap_frames"`
}

// The model types here mirror the database rows returned by queries.
// They are separate from the shared domain types (internal/shared) so the
// DB layer can be tested and evolved independently.

// User is a row from the users table.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	PasswordHash *string   `json:"-"`
	OIDCSub      *string   `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Agent is a row from the agents table.
type Agent struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Hostname      string     `json:"hostname"`
	IPAddress     string     `json:"ip_address"`
	Status        string     `json:"status"`
	Tags          []string   `json:"tags"`
	GPUVendor     string     `json:"gpu_vendor"`
	GPUModel      string     `json:"gpu_model"`
	GPUEnabled    bool       `json:"gpu_enabled"`
	AgentVersion  string     `json:"agent_version"`
	OSVersion     string     `json:"os_version"`
	CPUCount      int32      `json:"cpu_count"`
	RAMMIB        int64      `json:"ram_mib"`
	NVENC         bool       `json:"nvenc"`
	QSV           bool       `json:"qsv"`
	AMF           bool       `json:"amf"`
	// VNCPort is the TCP port the agent's VNC server is listening on.
	// 0 means VNC is not configured or not running on this agent.
	VNCPort       int        `json:"vnc_port"`
	APIKeyHash    *string    `json:"-"`
	LastHeartbeat *time.Time `json:"last_heartbeat,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Source is a row from the sources table.
type Source struct {
	ID         string     `json:"id"`
	Filename   string     `json:"filename"`
	UNCPath    string     `json:"path"`
	SizeBytes  int64      `json:"size_bytes"`
	DetectedBy *string    `json:"detected_by,omitempty"`
	State      string     `json:"state"`
	VMafScore  *float64   `json:"vmaf_score,omitempty"`
	// CloudURI is an optional cloud storage URI (s3://, gs://, az://) for
	// sources that live in object storage rather than on a UNC share.
	CloudURI *string `json:"cloud_uri,omitempty"`
	// HDR metadata — populated by hdr_detect analysis jobs.
	// HDRType: "hdr10", "hdr10+", "dolby_vision", "hlg", or "" for SDR/unknown.
	HDRType   string `json:"hdr_type"`
	// DVProfile: Dolby Vision profile number (5, 7, 8, 9 …) or 0 if no DV.
	DVProfile int    `json:"dv_profile"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Job is a row from the jobs table, enriched with source path via JOIN.
type Job struct {
	ID             string       `json:"id"`
	SourceID       string       `json:"source_id"`
	SourcePath     string       `json:"source_path"` // populated via LEFT JOIN sources
	Status         string       `json:"status"`
	JobType        string       `json:"job_type"`
	Priority       int          `json:"priority"`
	TargetTags     []string     `json:"target_tags"`
	TasksTotal     int          `json:"tasks_total"`
	TasksPending   int          `json:"tasks_pending"`
	TasksRunning   int          `json:"tasks_running"`
	TasksCompleted int          `json:"tasks_completed"`
	TasksFailed    int          `json:"tasks_failed"`
	EncodeConfig   EncodeConfig `json:"encode_config"`
	CompletedAt    *time.Time   `json:"completed_at,omitempty"`
	FailedAt       *time.Time   `json:"failed_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// TaskType constants identify the role of a task within a job.
// The empty string (TaskTypeEncode) is the default for backward compatibility.
const (
	TaskTypeEncode = ""       // standard per-chunk encode task
	TaskTypeConcat = "concat" // post-encode ffmpeg segment merge task
)

// Task is a row from the tasks table.
type Task struct {
	ID            string            `json:"id"`
	JobID         string            `json:"job_id"`
	ChunkIndex    int               `json:"chunk_index"`
	TaskType      string            `json:"task_type,omitempty"`
	Status        string            `json:"status"`
	AgentID       *string           `json:"agent_id,omitempty"`
	ScriptDir     string            `json:"-"`
	SourcePath    string            `json:"source_path,omitempty"`
	OutputPath    string            `json:"output_path,omitempty"`
	Variables     map[string]string `json:"-"`
	ExitCode      *int              `json:"exit_code,omitempty"`
	FramesEncoded *int64            `json:"frames_encoded,omitempty"`
	AvgFPS        *float64          `json:"avg_fps,omitempty"`
	OutputSize    *int64            `json:"output_size,omitempty"`
	DurationSec   *int64            `json:"duration_sec,omitempty"`
	VMafScore     *float64          `json:"vmaf_score,omitempty"`
	PSNR          *float64          `json:"psnr,omitempty"`
	SSIM          *float64          `json:"ssim,omitempty"`
	ErrorMsg      *string           `json:"error_msg,omitempty"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// TaskLog is a row from the task_logs table.
type TaskLog struct {
	ID       int64          `json:"id"`
	TaskID   string         `json:"task_id"`
	JobID    string         `json:"job_id"`
	Stream   string         `json:"stream"`
	Level    string         `json:"level"`
	Message  string         `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
	LoggedAt time.Time      `json:"timestamp"` // "timestamp" matches the frontend LogEntry type
}

// Template is a row from the templates table.
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Type        string    `json:"type"`
	Extension   string    `json:"extension,omitempty"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Variable is a row from the variables table.
type Variable struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Value       string    `json:"value"`
	Description string    `json:"description,omitempty"`
	Category    string    `json:"category,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Webhook is a row from the webhooks table.
type Webhook struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	URL       string    `json:"url"`
	Secret    *string   `json:"-"` // never serialised to clients
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookDelivery is a row from the webhook_deliveries table.
type WebhookDelivery struct {
	ID           int64     `json:"id"`
	WebhookID    string    `json:"webhook_id"`
	Event        string    `json:"event"`
	ResponseCode *int      `json:"response_code,omitempty"`
	Success      bool      `json:"success"`
	Attempt      int       `json:"attempt"`
	ErrorMsg     *string   `json:"error_msg,omitempty"`
	DeliveredAt  time.Time `json:"delivered_at"`
}

// AnalysisResult is a row from the analysis_results table.
type AnalysisResult struct {
	ID        string    `json:"id"`
	SourceID  string    `json:"source_id"`
	Type      string    `json:"type"`
	FrameData []byte    `json:"frame_data,omitempty"`
	Summary   []byte    `json:"summary,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// PathMapping is a row from the path_mappings table.
// It maps Windows UNC path prefixes (e.g. \\NAS01\media) to Linux mount
// path prefixes (e.g. /mnt/nas/media) so the controller can access NAS files
// via its local NFS mounts when running analysis jobs.
type PathMapping struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	WindowsPrefix string    `json:"windows_prefix"`
	LinuxPrefix   string    `json:"linux_prefix"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Session is a row from the sessions table.
type Session struct {
	Token     string    `json:"-"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// EnrollmentToken is a row from the enrollment_tokens table.
type EnrollmentToken struct {
	ID        string     `json:"id"`
	Token     string     `json:"token"`
	CreatedBy string     `json:"created_by"`
	UsedBy    *string    `json:"used_by,omitempty"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// AuditEntry is a row from the audit_log table.
type AuditEntry struct {
	ID         int64     `json:"id"`
	UserID     *string   `json:"user_id,omitempty"`
	Username   string    `json:"username"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id"`
	Detail     []byte    `json:"detail,omitempty"` // raw JSON
	IPAddress  string    `json:"ip_address"`
	LoggedAt   time.Time `json:"logged_at"`
}

// ---------------------------------------------------------------------------
// Parameter structs — one per write operation
// ---------------------------------------------------------------------------

type CreateUserParams struct {
	Username     string
	Email        string
	Role         string
	PasswordHash *string
	OIDCSub      *string
}

type UpsertAgentParams struct {
	Name         string
	Hostname     string
	IPAddress    string
	Tags         []string
	GPUVendor    string
	GPUModel     string
	GPUEnabled   bool
	AgentVersion string
	OSVersion    string
	CPUCount     int32
	RAMMIB       int64
	NVENC        bool
	QSV          bool
	AMF          bool
	VNCPort      int
}

type UpdateAgentHeartbeatParams struct {
	ID      string
	Status  string
	Metrics map[string]any
}

type CreateSourceParams struct {
	Filename   string
	UNCPath    string
	SizeBytes  int64
	DetectedBy *string
	CloudURI   *string
}

type UpdateSourceHDRParams struct {
	ID        string
	HDRType   string
	DVProfile int
}

type ListSourcesFilter struct {
	State    string
	Cursor   string
	PageSize int
}

type CreateJobParams struct {
	SourceID     string
	JobType      string
	Priority     int
	TargetTags   []string
	EncodeConfig EncodeConfig
}

type ListJobsFilter struct {
	Status   string
	Search   string
	Cursor   string
	PageSize int
}

type CreateTaskParams struct {
	JobID      string
	ChunkIndex int
	TaskType   string // empty string = TaskTypeEncode (default)
	SourcePath string
	OutputPath string
	Variables  map[string]string
}

type CompleteTaskParams struct {
	ID            string
	ExitCode      int
	FramesEncoded int64
	AvgFPS        float64
	OutputSize    int64
	DurationSec   int64
	VMafScore     *float64
	PSNR          *float64
	SSIM          *float64
}

type InsertTaskLogParams struct {
	TaskID   string
	JobID    string
	Stream   string
	Level    string
	Message  string
	Metadata map[string]any
	LoggedAt *time.Time
}

type ListTaskLogsParams struct {
	TaskID   string
	Stream   string
	Cursor   int64
	PageSize int
}

type CreateTemplateParams struct {
	Name        string
	Description string
	Type        string
	Extension   string
	Content     string
}

type UpdateTemplateParams struct {
	ID          string
	Name        string
	Description string
	Content     string
}

type UpsertVariableParams struct {
	Name        string
	Value       string
	Description string
	Category    string
}

type CreateWebhookParams struct {
	Name     string
	Provider string
	URL      string
	Secret   *string // raw HMAC-SHA256 signing key
	Events   []string
}

type UpdateWebhookParams struct {
	ID      string
	Name    string
	URL     string
	Events  []string
	Enabled bool
}

type InsertWebhookDeliveryParams struct {
	WebhookID    string
	Event        string
	Payload      []byte
	ResponseCode *int
	Success      bool
	Attempt      int
	ErrorMsg     *string
}

type UpsertAnalysisResultParams struct {
	SourceID  string
	Type      string
	FrameData []byte
	Summary   []byte
}

type CreateSessionParams struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
}

type ListJobLogsParams struct {
	JobID    string
	Stream   string
	Cursor   int64
	PageSize int
}

type CreateEnrollmentTokenParams struct {
	Token     string
	CreatedBy string
	ExpiresAt time.Time
}

type ConsumeEnrollmentTokenParams struct {
	Token   string
	AgentID string
}

type CreatePathMappingParams struct {
	Name          string
	WindowsPrefix string
	LinuxPrefix   string
}

type UpdatePathMappingParams struct {
	ID            string
	Name          string
	WindowsPrefix string
	LinuxPrefix   string
	Enabled       bool
}

type CreateAuditEntryParams struct {
	UserID     *string
	Username   string
	Action     string
	Resource   string
	ResourceID string
	Detail     []byte
	IPAddress  string
}

// Schedule is a row from the schedules table.
// JobTemplate holds the raw JSON that will be decoded into CreateJobParams
// when the scheduler fires.
type Schedule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	CronExpr    string          `json:"cron_expr"`
	JobTemplate json.RawMessage `json:"job_template"`
	Enabled     bool            `json:"enabled"`
	LastRunAt   *time.Time      `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time      `json:"next_run_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// APIKey is a row from the api_keys table.
// KeyHash stores the SHA-256 hash of the plaintext key; the plaintext is never
// persisted and is returned to the caller only at creation time.
type APIKey struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// CreateAPIKeyParams holds values for inserting a new api_keys row.
type CreateAPIKeyParams struct {
	UserID    string
	Name      string
	KeyHash   string
	ExpiresAt *time.Time
}

// CreateScheduleParams holds values for inserting a new schedule row.
type CreateScheduleParams struct {
	Name        string
	CronExpr    string
	JobTemplate json.RawMessage
	Enabled     bool
	NextRunAt   *time.Time
}

// UpdateScheduleParams holds values for updating an existing schedule row.
type UpdateScheduleParams struct {
	ID          string
	Name        string
	CronExpr    string
	JobTemplate json.RawMessage
	Enabled     bool
	NextRunAt   *time.Time
}

// MarkScheduleRunParams records a completed run and advances next_run_at.
type MarkScheduleRunParams struct {
	ID        string
	LastRunAt time.Time
	NextRunAt *time.Time
}

// AgentMetric is a row from the agent_metrics table.
type AgentMetric struct {
	ID         int64     `json:"id"`
	AgentID    string    `json:"agent_id"`
	CPUPct     float32   `json:"cpu_pct"`
	GPUPct     float32   `json:"gpu_pct"`
	MemPct     float32   `json:"mem_pct"`
	RecordedAt time.Time `json:"recorded_at"`
}

// InsertAgentMetricParams holds values for recording one metric sample.
type InsertAgentMetricParams struct {
	AgentID string
	CPUPct  float32
	GPUPct  float32
	MemPct  float32
}
