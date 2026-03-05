export interface Job {
  id: string
  source_id: string
  source_path: string
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
  frames_encoded: number
  avg_fps: number
  output_size: number
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

export interface AnalysisResult {
  id: string
  source_id: string
  vmaf_score: number | null
  psnr: number | null
  ssim: number | null
  width: number | null
  height: number | null
  duration_sec: number | null
  frame_count: number | null
  created_at: string
}
