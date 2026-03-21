import type { Job, Task, Agent, Source, Template, Variable, Webhook, WebhookDelivery, User, LogEntry, AnalysisResult, PathMapping, EnrollmentToken } from '../types'

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
  }
}

export const createJob = (body: CreateJobRequest) =>
  request<Job>('/jobs', { method: 'POST', body: JSON.stringify(body) })

export const cancelJob = (id: string) => request<void>(`/jobs/${id}/cancel`, { method: 'POST' })

export const retryJob = (id: string) => request<void>(`/jobs/${id}/retry`, { method: 'POST' })

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

export const createSource = (body: { path: string; name?: string }) =>
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

// Templates
export const listTemplates = () => request<Template[]>('/templates')

export const getTemplate = (id: string) => request<Template>(`/templates/${id}`)

export const createTemplate = (body: Partial<Template>) =>
  request<Template>('/templates', { method: 'POST', body: JSON.stringify(body) })

export const updateTemplate = (id: string, body: Partial<Template>) =>
  request<Template>(`/templates/${id}`, { method: 'PUT', body: JSON.stringify(body) })

export const deleteTemplate = (id: string) => request<void>(`/templates/${id}`, { method: 'DELETE' })

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
