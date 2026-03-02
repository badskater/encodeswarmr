import type { Job, Task, Agent, Source, Template, Variable, Webhook, User, LogEntry, AnalysisResult } from '../types'

const API_BASE = '/api/v1'

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

// Jobs
export const listJobs = (status?: string) =>
  request<Job[]>(`/jobs${status ? `?status=${status}` : ''}`)

export const getJob = (id: string) => request<Job>(`/jobs/${id}`)

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

// Sources
export const listSources = () => request<Source[]>('/sources')

export const getSource = (id: string) => request<Source>(`/sources/${id}`)

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

export const deleteWebhook = (id: string) => request<void>(`/webhooks/${id}`, { method: 'DELETE' })

export const testWebhook = (id: string) => request<void>(`/webhooks/${id}/test`, { method: 'POST' })

// Users
export const listUsers = () => request<User[]>('/users')

export const createUser = (body: { username: string; email: string; role: string; password: string }) =>
  request<User>('/users', { method: 'POST', body: JSON.stringify(body) })

export const deleteUser = (id: string) => request<void>(`/users/${id}`, { method: 'DELETE' })

export const updateUserRole = (id: string, role: string) =>
  request<void>(`/users/${id}/role`, { method: 'PUT', body: JSON.stringify({ role }) })
