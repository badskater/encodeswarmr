import type { Job, Task, Agent, Source, Template, Variable, Webhook, WebhookDelivery, User, LogEntry, AnalysisResult, PathMapping, EnrollmentToken, SceneData, Schedule, ThroughputPoint, QueueSummary, ActivityEvent, Plugin, NotificationPrefs, AutoScalingSettings } from '../types'
import type { Job, Task, Agent, Source, Template, Variable, Webhook, WebhookDelivery, User, LogEntry, AnalysisResult, PathMapping, EnrollmentToken, SceneData, Schedule, ThroughputPoint, QueueSummary, ActivityEvent, Plugin, AudioConfig } from '../types'
import type { Flow } from '../types/flow'

const API_BASE = '/api/v1'

// Setup wizard — unauthenticated endpoints outside /api/v1
export const getSetupStatus = (): Promise<{ required: boolean }> =>
  fetch('/setup/status', { credentials: 'include' })
    .then(r => r.json())
    .then((j: { required?: boolean }) => ({ required: j.required ?? false }))

export const postSetup = (body: { username: string; email: string; password: string }) =>
  fetch('/setup', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify(body),
  })

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const resp = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    credentials: 'include',
  })
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}))
    throw new ApiError(resp.status, body.detail ?? resp.statusText)
  }
  const json = await resp.json()
  return (json.data ?? json) as T
}

// Auth
export const login = (username: string, password: string) =>
  fetch('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ username, password }),
  })

export const logout = () => fetch('/auth/logout', { method: 'POST', credentials: 'include' })

export const getMe = () => request<User>('/users/me')

async function requestCollection<T>(path: string): Promise<{ items: T[]; total: number; nextCursor?: string }> {
  const resp = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
  })
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({}))
    throw new ApiError(resp.status, body.detail ?? resp.statusText)
  }
  const json = await resp.json()
  return {
    items: (json.data ?? []) as T[],
    total: json.meta?.total_count ?? 0,
    nextCursor: json.meta?.next_cursor,
  }
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const p = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') p.set(k, String(v))
  }
  const s = p.toString()
  return s ? `?${s}` : ''
}

// Jobs
export const listJobs = (status?: string, search?: string) =>
  request<Job[]>(`/jobs${buildQuery({ status, search })}`)

export const listJobsPaged = (params: { status?: string; search?: string; cursor?: string; page_size?: number }) =>
  requestCollection<Job>(`/jobs${buildQuery(params)}`)

export const getJob = (id: string) => request<{ job: Job; tasks: Task[] }>(`/jobs/${id}`)

export interface CreateJobRequest {
  source_id: string
  job_type: string
  priority?: number
  target_tags?: string[]
  encode_config?: {
    run_script_template_id?: string
    frameserver_template_id?: string
    chunk_boundaries?: { start_frame: number; end_frame: number }[]
    output_root?: string
    output_extension?: string
    extra_vars?: Record<string, string>
    chunking_config?: {
      enable_chunking: boolean
      chunk_size_frames: number
      overlap_frames: number
    }
  }
  audio_config?: AudioConfig
  depends_on?: string
  chain_group?: string
  flow_id?: string
}

export const createJob = (body: CreateJobRequest) =>
  request<Job>('/jobs', { method: 'POST', body: JSON.stringify(body) })

export const cancelJob = (id: string) => request<void>(`/jobs/${id}/cancel`, { method: 'POST' })

export const retryJob = (id: string) => request<void>(`/jobs/${id}/retry`, { method: 'POST' })

// Job Chains

export interface ChainStep {
  job_type: string
  name?: string
  priority?: number
  target_tags?: string[]
  encode_config?: CreateJobRequest['encode_config']
  audio_config?: AudioConfig
}

export interface CreateJobChainRequest {
  source_id: string
  steps: ChainStep[]
}

export interface JobChainResponse {
  chain_group: string
  jobs: Job[]
}

export const createJobChain = (body: CreateJobChainRequest) =>
  request<JobChainResponse>('/job-chains', { method: 'POST', body: JSON.stringify(body) })

export const getJobChain = (chainGroup: string) =>
  request<JobChainResponse>(`/job-chains/${chainGroup}`)

// Batch Import

export interface BatchImportRequest {
  path_pattern: string
  recursive?: boolean
  auto_analyze?: boolean
  auto_encode?: boolean
  encode_template_id?: string
}

export interface BatchImportResult {
  path: string
  source_id?: string
  job_id?: string
  skipped?: boolean
  skip_reason?: string
  error?: string
}

export interface BatchImportResponse {
  imported: number
  results: BatchImportResult[]
}

export const batchImportSources = (body: BatchImportRequest) =>
  request<BatchImportResponse>('/sources/batch-import', { method: 'POST', body: JSON.stringify(body) })
/** Returns a URL for the job export endpoint (opens as file download). */
export const jobExportURL = (params: { format: 'csv' | 'json'; status?: string; from?: string; to?: string }) =>
  `${API_BASE}/jobs/export${buildQuery(params)}`

export const listArchivedJobs = (params?: { status?: string; cursor?: string; page_size?: number }) =>
  requestCollection<Job>(`/archive/jobs${buildQuery(params ?? {})}`)

/** Returns a URL for the archived job export endpoint. */
export const archivedJobExportURL = (params: { format: 'csv' | 'json'; status?: string; from?: string; to?: string }) =>
  `${API_BASE}/archive/jobs/export${buildQuery(params)}`

// Tasks
export const getTask = (id: string) => request<Task>(`/tasks/${id}`)

export const listTaskLogs = (id: string) => request<LogEntry[]>(`/tasks/${id}/logs`)

export const getTaskLogsTailURL = (id: string) => `${API_BASE}/tasks/${id}/logs/tail`

// Agents
export const listAgents = () => request<Agent[]>('/agents')

export const getAgent = (id: string) => request<Agent>(`/agents/${id}`)

export const drainAgent = (id: string) => request<void>(`/agents/${id}/drain`, { method: 'POST' })

export const approveAgent = (id: string) => request<void>(`/agents/${id}/approve`, { method: 'POST' })

// Sources
export const listSources = (params?: { state?: string; cursor?: string; page_size?: number }) =>
  request<Source[]>(`/sources${buildQuery(params ?? {})}`)

export const listSourcesPaged = (params: { state?: string; cursor?: string; page_size?: number }) =>
  requestCollection<Source>(`/sources${buildQuery(params)}`)

export const createSource = (body: { path?: string; name?: string; cloud_uri?: string }) =>
  request<Source>('/sources', { method: 'POST', body: JSON.stringify(body) })

export const getSource = (id: string) => request<Source>(`/sources/${id}`)

export const analyzeSource = (id: string) => request<Job>(`/sources/${id}/analyze`, { method: 'POST' })

export const hdrDetectSource = (id: string) =>
  request<{ job_id: string; source_id: string; status: string }>(`/sources/${id}/hdr-detect`, { method: 'POST' })

export const updateSourceHDR = (id: string, hdr_type: string, dv_profile: number) =>
  request<Source>(`/sources/${id}/hdr`, {
    method: 'PATCH',
    body: JSON.stringify({ hdr_type, dv_profile }),
  })

export const deleteSource = (id: string) => request<void>(`/sources/${id}`, { method: 'DELETE' })

export const getAnalysis = (sourceId: string) => request<AnalysisResult>(`/analysis/${sourceId}`)

export const listAnalysisResults = (sourceId: string) =>
  request<AnalysisResult[]>(`/analysis/${sourceId}/all`)

export const getSourceScenes = (sourceId: string) =>
  request<SceneData>(`/sources/${sourceId}/scenes`)

// Templates
export const listTemplates = () => request<Template[]>('/templates')

export const getTemplate = (id: string) => request<Template>(`/templates/${id}`)

export const createTemplate = (body: Partial<Template>) =>
  request<Template>('/templates', { method: 'POST', body: JSON.stringify(body) })

export const updateTemplate = (id: string, body: Partial<Template>) =>
  request<Template>(`/templates/${id}`, { method: 'PUT', body: JSON.stringify(body) })

export const deleteTemplate = (id: string) => request<void>(`/templates/${id}`, { method: 'DELETE' })

export const previewTemplate = (id: string, sourceId?: string, variables?: Record<string, string>) =>
  request<{ template_name: string; extension: string; content: string }>(
    `/templates/${id}/preview`,
    { method: 'POST', body: JSON.stringify({ source_id: sourceId ?? '', variables: variables ?? {} }) }
  )

// Variables
export const listVariables = () => request<Variable[]>('/variables')

export const upsertVariable = (name: string, value: string, description?: string) =>
  request<Variable>(`/variables/${name}`, {
    method: 'PUT',
    body: JSON.stringify({ value, description }),
  })

export const deleteVariable = (id: string) => request<void>(`/variables/${id}`, { method: 'DELETE' })

// Webhooks
export const listWebhooks = () => request<Webhook[]>('/webhooks')

export const createWebhook = (body: Partial<Webhook>) =>
  request<Webhook>('/webhooks', { method: 'POST', body: JSON.stringify(body) })

export const updateWebhook = (id: string, body: Partial<Webhook> & { enabled?: boolean }) =>
  request<Webhook>(`/webhooks/${id}`, { method: 'PUT', body: JSON.stringify(body) })

export const deleteWebhook = (id: string) => request<void>(`/webhooks/${id}`, { method: 'DELETE' })

export const testWebhook = (id: string) => request<void>(`/webhooks/${id}/test`, { method: 'POST' })

export const listWebhookDeliveries = (id: string, limit = 50, offset = 0) =>
  request<WebhookDelivery[]>(`/webhooks/${id}/deliveries${buildQuery({ limit, offset })}`)

// Users
export const listUsers = () => request<User[]>('/users')

export const createUser = (body: { username: string; email: string; role: string; password: string }) =>
  request<User>('/users', { method: 'POST', body: JSON.stringify(body) })

export const deleteUser = (id: string) => request<void>(`/users/${id}`, { method: 'DELETE' })

export const updateUserRole = (id: string, role: string) =>
  request<void>(`/users/${id}/role`, { method: 'PUT', body: JSON.stringify({ role }) })

// Path Mappings
export const listPathMappings = () => request<PathMapping[]>('/path-mappings')

export const createPathMapping = (body: { name: string; windows_prefix: string; linux_prefix: string; enabled?: boolean }) =>
  request<PathMapping>('/path-mappings', { method: 'POST', body: JSON.stringify(body) })

export const updatePathMapping = (id: string, body: Partial<PathMapping>) =>
  request<PathMapping>(`/path-mappings/${id}`, { method: 'PUT', body: JSON.stringify(body) })

export const deletePathMapping = (id: string) =>
  request<void>(`/path-mappings/${id}`, { method: 'DELETE' })

// Enrollment Tokens
export const listEnrollmentTokens = () => request<EnrollmentToken[]>('/agent-tokens')

export const createEnrollmentToken = (body?: { expires_at?: string }) =>
  request<EnrollmentToken>('/agent-tokens', { method: 'POST', body: JSON.stringify(body ?? {}) })

export const deleteEnrollmentToken = (id: string) =>
  request<void>(`/agent-tokens/${id}`, { method: 'DELETE' })

// Agent Metrics
export interface AgentMetric {
  id: number
  agent_id: string
  cpu_pct: number
  gpu_pct: number
  mem_pct: number
  recorded_at: string
}

export const listAgentMetrics = (agentId: string, window = '1h') =>
  request<AgentMetric[]>(`/agents/${agentId}/metrics${buildQuery({ window })}`)

// Schedules
export const listSchedules = () => request<Schedule[]>('/schedules')

export const getSchedule = (id: string) => request<Schedule>(`/schedules/${id}`)

export const createSchedule = (body: { name: string; cron_expr: string; job_template: unknown; enabled?: boolean }) =>
  request<Schedule>('/schedules', { method: 'POST', body: JSON.stringify(body) })

export const updateSchedule = (id: string, body: { name: string; cron_expr: string; job_template: unknown; enabled?: boolean }) =>
  request<Schedule>(`/schedules/${id}`, { method: 'PUT', body: JSON.stringify(body) })

export const deleteSchedule = (id: string) => request<void>(`/schedules/${id}`, { method: 'DELETE' })

// Dashboard metrics
export const getThroughput = (hours = 24) =>
  request<ThroughputPoint[]>(`/metrics/throughput${buildQuery({ hours })}`)

export const getQueueSummary = () =>
  request<QueueSummary>('/metrics/queue')

export const getRecentActivity = (limit = 10) =>
  request<ActivityEvent[]>(`/metrics/activity${buildQuery({ limit })}`)

// Plugins
export const listPlugins = () => request<Plugin[]>('/plugins')

export const togglePlugin = (name: string, enable: boolean) =>
  request<Plugin>(`/plugins/${name}/${enable ? 'enable' : 'disable'}`, { method: 'PUT' })

// Flows
export const listFlows = () => request<Flow[]>('/flows')

export const getFlow = (id: string) => request<Flow>(`/flows/${id}`)

export const createFlow = (data: Partial<Flow>) =>
  request<Flow>('/flows', { method: 'POST', body: JSON.stringify(data) })

export const updateFlow = (id: string, data: Partial<Flow>) =>
  request<Flow>(`/flows/${id}`, { method: 'PUT', body: JSON.stringify(data) })

export const deleteFlow = (id: string) =>
  request<void>(`/flows/${id}`, { method: 'DELETE' })

// Audit Log
export interface AuditEntry {
  id: number
  user_id: string | null
  username: string
  action: string
  resource: string
  resource_id: string
  ip_address: string
  logged_at: string
}

export const listAuditLog = (limit = 100, offset = 0) =>
  requestCollection<AuditEntry>(`/audit-log${buildQuery({ limit, offset })}`)

// Notification Preferences
export const getNotificationPrefs = () =>
  request<NotificationPrefs>('/me/notifications')

export const updateNotificationPrefs = (body: Partial<NotificationPrefs>) =>
  request<NotificationPrefs>('/me/notifications', { method: 'PUT', body: JSON.stringify(body) })

export const testEmail = (to: string) =>
  request<{ ok: boolean; to: string }>('/notifications/test-email', {
    method: 'POST',
    body: JSON.stringify({ to }),
  })

// Auto-Scaling Settings
export const getAutoScaling = () =>
  request<AutoScalingSettings>('/settings/auto-scaling')

export const updateAutoScaling = (body: Partial<AutoScalingSettings>) =>
  request<AutoScalingSettings>('/settings/auto-scaling', {
    method: 'PUT',
    body: JSON.stringify(body),
  })

export const testAutoScalingWebhook = () =>
  request<{ ok: boolean; url: string }>('/settings/auto-scaling/test', { method: 'POST' })
