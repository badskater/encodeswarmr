export interface Job {
  id: string
  source_id: string
  source_path: string
  job_type: string
  status: 'queued' | 'assigned' | 'running' | 'completed' | 'failed' | 'cancelled'
  priority: number
  tasks_total: number
  tasks_completed: number
  tasks_failed: number
  tasks_pending: number
  tasks_running: number
  created_at: string
  updated_at: string
}

export interface Task {
  id: string
  job_id: string
  agent_id: string | null
  status: 'pending' | 'assigned' | 'running' | 'completed' | 'failed'
  chunk_index: number
  exit_code: number | null
  error_msg: string | null
  frames_encoded: number | null
  avg_fps: number | null
  output_size: number | null
  started_at: string | null
  completed_at: string | null
  created_at: string
}

export interface Agent {
  id: string
  name: string
  hostname: string
  ip_address: string
  status: 'idle' | 'running' | 'offline' | 'draining' | 'pending_approval'
  tags: string[]
  agent_version: string
  os_version: string
  cpu_count: number
  ram_mib: number
  gpu_vendor: string | null
  gpu_model: string | null
  gpu_enabled: boolean
  nvenc: boolean
  qsv: boolean
  amf: boolean
  // vnc_port is non-zero when the agent has VNC configured and running.
  vnc_port: number
  last_heartbeat: string | null
  created_at: string
}

export interface Source {
  id: string
  path: string
  filename: string
  size_bytes: number
  duration_sec: number | null
  state: 'new' | 'analysing' | 'ready' | 'encoding' | 'done' | 'error'
  vmaf_score: number | null
  cloud_uri: string | null
  hdr_type: string
  dv_profile: number
  created_at: string
}

export interface Template {
  id: string
  name: string
  type: 'avs' | 'vpy' | 'bat'
  content: string
  description: string | null
  created_at: string
  updated_at: string
}

export interface Variable {
  id: string
  name: string
  value: string
  description: string | null
  created_at: string
  updated_at: string
}

export interface Webhook {
  id: string
  name: string
  provider: 'discord' | 'teams' | 'slack' | 'generic'
  url: string
  events: string[]
  enabled: boolean
  created_at: string
}

export interface WebhookDelivery {
  id: number
  webhook_id: string
  event: string
  response_code: number | null
  success: boolean
  attempt: number
  error_msg: string | null
  delivered_at: string
}

export interface User {
  id: string
  username: string
  email: string
  role: 'admin' | 'operator' | 'viewer'
  created_at: string
}

export interface LogEntry {
  id: string
  task_id: string
  stream: 'stdout' | 'stderr' | 'agent'
  level: string
  message: string
  timestamp: string
}

export interface AnalysisFramePoint {
  frame: number
  score?: number
  pts?: number
}

export interface AnalysisSummary {
  mean?: number
  min?: number
  max?: number
  psnr?: number
  ssim?: number
  width?: number
  height?: number
  duration_sec?: number
  frame_count?: number
  codec?: string
  bit_rate?: number
  scene_count?: number
}

export interface AnalysisResult {
  id: string
  source_id: string
  type: string
  frame_data?: AnalysisFramePoint[] | null
  summary?: AnalysisSummary | null
  created_at: string
}

export interface PathMapping {
  id: string
  name: string
  windows_prefix: string
  linux_prefix: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface EnrollmentToken {
  id: string
  token: string
  created_by: string
  used_by: string | null
  used_at: string | null
  expires_at: string | null
  created_at: string
}

// SceneBoundary is a single scene cut as returned by GET /api/v1/sources/{id}/scenes.
export interface SceneBoundary {
  frame: number
  pts: number
  timecode: string
}

// SceneData is the response envelope from GET /api/v1/sources/{id}/scenes.
export interface SceneData {
  source_id: string
  fps: number
  total_frames: number
  duration_sec: number
  scenes: SceneBoundary[]
}

// ChunkingConfig holds the optional scene-based auto-chunking parameters
// sent to the job creation API when the operator enables chunked encoding.
export interface ChunkingConfig {
  enable_chunking: boolean
  chunk_size_frames: number
  overlap_frames: number
}

// Schedule is a row from the schedules table.
// job_template is the raw JSON object that will be decoded into a CreateJobParams
// when the schedule fires.
export interface Schedule {
  id: string
  name: string
  cron_expr: string
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  job_template: Record<string, any>
  enabled: boolean
  last_run_at: string | null
  next_run_at: string | null
  created_at: string
}

// ThroughputPoint is a single hour bucket returned by the metrics/throughput endpoint.
export interface ThroughputPoint {
  hour: string  // ISO timestamp of the start of the hour bucket
  completed: number
}

// QueueSummary summarises the current task queue depth.
export interface QueueSummary {
  pending: number
  running: number
  estimated_completion_sec: number | null
}

// ActivityEvent is a single entry in the recent job activity feed.
export interface ActivityEvent {
  job_id: string
  source_path: string
  status: string
  changed_at: string
}

// Plugin represents an installed encoding plugin.
export interface Plugin {
  id: string
  name: string
  version: string
  description: string
  enabled: boolean
  author: string | null
}
