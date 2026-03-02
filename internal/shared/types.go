package shared

import "time"

type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusAssigned  JobStatus = "assigned"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusAssigned  TaskStatus = "assigned"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

type AgentStatus string

const (
	AgentStatusPendingApproval AgentStatus = "pending_approval"
	AgentStatusIdle            AgentStatus = "idle"
	AgentStatusBusy            AgentStatus = "busy"
	AgentStatusOffline         AgentStatus = "offline"
	AgentStatusDisabled        AgentStatus = "disabled"
)

type JobType string

const (
	JobTypeEncode   JobType = "encode"
	JobTypeAnalysis JobType = "analysis"
	JobTypeAudio    JobType = "audio"
)

type AnalysisType string

const (
	AnalysisTypeHistogram   AnalysisType = "histogram"
	AnalysisTypeVMAF        AnalysisType = "vmaf"
	AnalysisTypeSceneDetect AnalysisType = "scene_detect"
)

type WebhookProvider string

const (
	WebhookProviderDiscord WebhookProvider = "discord"
	WebhookProviderTeams   WebhookProvider = "teams"
	WebhookProviderSlack   WebhookProvider = "slack"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// Job represents an encoding job.
type Job struct {
	ID          string     `json:"id"`
	SourceID    string     `json:"source_id"`
	Status      JobStatus  `json:"status"`
	Type        JobType    `json:"job_type"`
	Priority    int        `json:"priority"`
	TargetTags  []string   `json:"target_tags"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	FailedAt    *time.Time `json:"failed_at,omitempty"`
}

// Task represents a single chunk task within a job.
type Task struct {
	ID          string            `json:"id"`
	JobID       string            `json:"job_id"`
	ChunkIndex  int               `json:"chunk_index"`
	Status      TaskStatus        `json:"status"`
	AgentID     *string           `json:"agent_id,omitempty"`
	ScriptDir   string            `json:"script_dir"`
	SourcePath  string            `json:"source_path"`
	OutputPath  string            `json:"output_path"`
	Variables   map[string]string `json:"variables"`
	ExitCode    *int              `json:"exit_code,omitempty"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Agent represents a registered Windows encoding agent.
type Agent struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Hostname   string      `json:"hostname"`
	IPAddress  string      `json:"ip_address"`
	Status     AgentStatus `json:"status"`
	Tags       []string    `json:"tags"`
	GPUVendor  string      `json:"gpu_vendor"`
	GPUModel   string      `json:"gpu_model"`
	GPUEnabled bool        `json:"gpu_enabled"`
	LastSeen   *time.Time  `json:"last_seen,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// Source represents a detected source video file.
type Source struct {
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	UNCPath    string    `json:"unc_path"`
	SizeBytes  int64     `json:"size_bytes"`
	DetectedBy string    `json:"detected_by"`
	State      string    `json:"state"` // detected, scanning, ready, encoding, done
	VMafScore  *float64  `json:"vmaf_score,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Template represents a script template.
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        string    `json:"type"`      // run_script, frameserver
	Extension   string    `json:"extension"` // bat, avs, vpy
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Variable represents a global key-value variable injected into script templates.
type Variable struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WebhookEndpoint represents a configured webhook target.
type WebhookEndpoint struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Provider  WebhookProvider `json:"provider"`
	URL       string          `json:"url"`
	Secret    string          `json:"-"`
	Events    []string        `json:"events"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// User represents a local or OIDC-authenticated user.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	Role         Role      `json:"role"`
	PasswordHash *string   `json:"-"`
	OIDCSub      *string   `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AnalysisResult stores results from video analysis jobs.
type AnalysisResult struct {
	ID        string       `json:"id"`
	SourceID  string       `json:"source_id"`
	Type      AnalysisType `json:"type"`
	FrameData []byte       `json:"frame_data"` // JSONB
	Summary   []byte       `json:"summary"`    // JSONB
	CreatedAt time.Time    `json:"created_at"`
}

// Webhook event name constants.
const (
	EventJobCompleted    = "job.completed"
	EventJobFailed       = "job.failed"
	EventJobCancelled    = "job.cancelled"
	EventTaskFailed      = "task.failed"
	EventAgentOnline     = "agent.online"
	EventAgentOffline    = "agent.offline"
	EventAgentRegistered = "agent.registered"
	EventSourceDetected  = "source.detected"
	EventSourceScanned   = "source.scanned"
)
