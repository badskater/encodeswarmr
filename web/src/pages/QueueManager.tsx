import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import type { Job, QueueStatus, Agent } from '../types'
import StatusBadge from '../components/StatusBadge'
import { useAutoRefresh } from '../hooks/useAutoRefresh'

function basename(p: string) {
  return p.split(/[\\/]/).pop() ?? p
}

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

interface PriorityCell {
  jobId: string
  priority: number
  onSave: (id: string, p: number) => void
}

function PriorityCell({ jobId, priority, onSave }: PriorityCell) {
  const [editing, setEditing] = useState(false)
  const [val, setVal] = useState(String(priority))

  const save = () => {
    const n = parseInt(val, 10)
    if (!isNaN(n) && n >= 0 && n <= 100) onSave(jobId, n)
    setEditing(false)
  }

  if (editing) {
    return (
      <input
        type="number"
        min={0}
        max={100}
        value={val}
        onChange={e => setVal(e.target.value)}
        onBlur={save}
        onKeyDown={e => { if (e.key === 'Enter') save(); if (e.key === 'Escape') setEditing(false) }}
        className="w-16 bg-th-input-bg border border-th-input-border rounded px-1 py-0.5 text-sm text-th-text"
        autoFocus
      />
    )
  }

  return (
    <button
      onClick={() => { setVal(String(priority)); setEditing(true) }}
      className="text-th-text hover:underline cursor-pointer text-sm"
      title="Click to edit priority (0–100)"
    >
      {priority}
    </button>
  )
}

export default function QueueManager() {
  const navigate = useNavigate()
  const [queueStatus, setQueueStatus] = useState<QueueStatus | null>(null)
  const [pendingJobs, setPendingJobs] = useState<Job[]>([])
  const [runningJobs, setRunningJobs] = useState<Job[]>([])
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [toggling, setToggling] = useState(false)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [bulkCancelling, setBulkCancelling] = useState(false)

  const load = useCallback(async () => {
    try {
      const [status, agentList, pendingResult, runningResult] = await Promise.all([
        api.getQueueStatus(),
        api.listAgents(),
        api.listJobsPaged({ status: 'queued', page_size: 200 }),
        api.listJobsPaged({ status: 'running', page_size: 50 }),
      ])
      setQueueStatus(status)
      setAgents(agentList)
      setPendingJobs(pendingResult.items)
      setRunningJobs(runningResult.items)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])
  useAutoRefresh(load)

  const handleTogglePause = async () => {
    if (!queueStatus) return
    setToggling(true)
    try {
      if (queueStatus.paused) {
        await api.resumeQueue()
      } else {
        await api.pauseQueue()
      }
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to toggle queue')
    } finally {
      setToggling(false)
    }
  }

  const handleMoveUp = async (id: string) => {
    const idx = pendingJobs.findIndex(j => j.id === id)
    if (idx <= 0) return
    const above = pendingJobs[idx - 1]
    const current = pendingJobs[idx]
    try {
      await Promise.all([
        api.updateJobPriority(above.id, current.priority),
        api.updateJobPriority(current.id, above.priority),
      ])
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to reorder')
    }
  }

  const handleMoveDown = async (id: string) => {
    const idx = pendingJobs.findIndex(j => j.id === id)
    if (idx < 0 || idx >= pendingJobs.length - 1) return
    const below = pendingJobs[idx + 1]
    const current = pendingJobs[idx]
    try {
      await Promise.all([
        api.updateJobPriority(below.id, current.priority),
        api.updateJobPriority(current.id, below.priority),
      ])
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to reorder')
    }
  }

  const handlePriorityEdit = async (id: string, priority: number) => {
    try {
      await api.updateJobPriority(id, priority)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to update priority')
    }
  }

  const handleCancel = async (id: string) => {
    try {
      await api.cancelJob(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to cancel job')
    }
  }

  const handleBulkCancel = async () => {
    if (selected.size === 0) return
    setBulkCancelling(true)
    setError('')
    try {
      for (const id of selected) {
        await api.cancelJob(id)
      }
      setSelected(new Set())
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to bulk cancel')
    } finally {
      setBulkCancelling(false)
    }
  }

  const toggleSelect = (id: string) => {
    setSelected(s => {
      const n = new Set(s)
      if (n.has(id)) n.delete(id); else n.add(id)
      return n
    })
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h1 className="text-2xl font-bold text-th-text">Queue Manager</h1>
        <button
          onClick={handleTogglePause}
          disabled={toggling}
          className={`px-4 py-1.5 rounded text-sm font-medium disabled:opacity-50 ${
            queueStatus?.paused
              ? 'bg-green-600 text-white hover:bg-green-700'
              : 'bg-amber-600 text-white hover:bg-amber-700'
          }`}
        >
          {toggling ? '…' : queueStatus?.paused ? 'Resume Queue' : 'Pause Queue'}
        </button>
      </div>

      {/* Status bar */}
      <div className="bg-th-surface rounded-lg shadow px-4 py-3 flex flex-wrap gap-6 text-sm">
        <div>
          <span className="text-th-text-muted">Status: </span>
          {queueStatus?.paused ? (
            <span className="font-medium text-amber-600">Paused</span>
          ) : (
            <span className="font-medium text-green-600">Running</span>
          )}
        </div>
        <div>
          <span className="text-th-text-muted">Pending: </span>
          <span className="font-medium text-th-text">{queueStatus?.pending ?? 0}</span>
        </div>
        <div>
          <span className="text-th-text-muted">Running: </span>
          <span className="font-medium text-th-text">{queueStatus?.running ?? 0}</span>
        </div>
        {queueStatus?.estimated_completion && (
          <div>
            <span className="text-th-text-muted">Est. completion: </span>
            <span className="font-medium text-th-text">~{queueStatus.estimated_completion}</span>
          </div>
        )}
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {/* Running jobs */}
      {runningJobs.length > 0 && (
        <div className="bg-th-surface rounded-lg shadow overflow-hidden">
          <div className="px-4 py-2 border-b border-th-border bg-th-surface-muted">
            <h2 className="text-xs font-semibold text-th-text-muted uppercase tracking-wide">
              Running ({runningJobs.length})
            </h2>
          </div>
          <div className="divide-y divide-th-border-subtle">
            {runningJobs.map(job => {
              const progress = job.tasks_total > 0 ? (job.tasks_completed / job.tasks_total) * 100 : 0
              return (
                <div key={job.id} className="px-4 py-2 flex items-center gap-3 cursor-pointer hover:bg-th-surface-muted" onClick={() => navigate(`/jobs/${job.id}`)}>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="font-mono text-xs text-blue-600">{job.id.slice(0, 8)}</span>
                      <span className="text-sm text-th-text-secondary truncate">{basename(job.source_path)}</span>
                      {job.target_tags?.length ? (
                        <span className="text-xs text-th-text-subtle">({job.target_tags.join(', ')})</span>
                      ) : null}
                      {job.eta_human && (
                        <span className="text-xs text-th-text-muted ml-auto whitespace-nowrap">~{job.eta_human}</span>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <div className="flex-1 h-2 bg-th-surface-muted rounded-full overflow-hidden max-w-xs">
                        <div className="h-full bg-blue-500 rounded-full transition-all" style={{ width: `${progress}%` }} />
                      </div>
                      <span className="text-xs text-th-text-muted whitespace-nowrap">
                        {job.tasks_completed}/{job.tasks_total}
                      </span>
                    </div>
                  </div>
                  <StatusBadge status={job.status} />
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Pending queue */}
      <div className="bg-th-surface rounded-lg shadow overflow-hidden">
        <div className="px-4 py-2 border-b border-th-border bg-th-surface-muted flex items-center justify-between">
          <h2 className="text-xs font-semibold text-th-text-muted uppercase tracking-wide">
            Pending Queue ({pendingJobs.length})
          </h2>
          {selected.size > 0 && (
            <button
              onClick={handleBulkCancel}
              disabled={bulkCancelling}
              className="text-xs px-3 py-1 rounded font-medium disabled:opacity-50"
              style={{ backgroundColor: 'var(--th-badge-error-bg)', color: 'var(--th-badge-error-text)' }}
            >
              {bulkCancelling ? 'Cancelling…' : `Cancel Selected (${selected.size})`}
            </button>
          )}
        </div>

        {pendingJobs.length === 0 ? (
          <p className="px-4 py-6 text-center text-th-text-subtle text-sm">No jobs pending.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm divide-y divide-th-border">
              <thead className="bg-th-surface-muted">
                <tr>
                  <th className="px-2 py-2 w-10" />
                  <th className="px-2 py-2 w-8">
                    <input
                      type="checkbox"
                      checked={pendingJobs.length > 0 && pendingJobs.every(j => selected.has(j.id))}
                      onChange={e => {
                        if (e.target.checked) setSelected(new Set(pendingJobs.map(j => j.id)))
                        else setSelected(new Set())
                      }}
                      className="rounded border-th-input-border"
                    />
                  </th>
                  {['ID', 'Source', 'Status', 'Priority', 'Target Tags', 'Created', ''].map(h => (
                    <th key={h} className="px-3 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-th-border-subtle">
                {pendingJobs.map((job, idx) => (
                  <tr key={job.id} className="hover:bg-th-surface-muted">
                    <td className="px-2 py-2 text-center">
                      <div className="flex flex-col gap-0.5">
                        <button
                          onClick={() => handleMoveUp(job.id)}
                          disabled={idx === 0}
                          className="text-th-text-muted hover:text-th-text disabled:opacity-20 text-xs leading-none"
                        >▲</button>
                        <button
                          onClick={() => handleMoveDown(job.id)}
                          disabled={idx === pendingJobs.length - 1}
                          className="text-th-text-muted hover:text-th-text disabled:opacity-20 text-xs leading-none"
                        >▼</button>
                      </div>
                    </td>
                    <td className="px-2 py-2">
                      <input
                        type="checkbox"
                        checked={selected.has(job.id)}
                        onChange={() => toggleSelect(job.id)}
                        className="rounded border-th-input-border"
                      />
                    </td>
                    <td
                      className="px-3 py-2 font-mono text-blue-600 cursor-pointer"
                      onClick={() => navigate(`/jobs/${job.id}`)}
                    >
                      {job.id.slice(0, 8)}
                    </td>
                    <td
                      className="px-3 py-2 max-w-xs truncate text-th-text-secondary cursor-pointer"
                      onClick={() => navigate(`/jobs/${job.id}`)}
                    >
                      {basename(job.source_path)}
                    </td>
                    <td className="px-3 py-2">
                      <StatusBadge status={job.status} />
                    </td>
                    <td className="px-3 py-2">
                      <PriorityCell jobId={job.id} priority={job.priority} onSave={handlePriorityEdit} />
                    </td>
                    <td className="px-3 py-2 text-xs text-th-text-muted">
                      {job.target_tags?.length ? job.target_tags.join(', ') : '—'}
                    </td>
                    <td className="px-3 py-2 text-xs text-th-text-muted whitespace-nowrap">
                      {fmtDate(job.created_at)}
                    </td>
                    <td className="px-3 py-2">
                      <button
                        onClick={() => handleCancel(job.id)}
                        className="text-xs px-2 py-0.5 rounded text-red-600 border border-red-200 hover:bg-red-50"
                      >
                        Cancel
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Agent status (for context) */}
      {agents.length > 0 && (
        <div className="bg-th-surface rounded-lg shadow overflow-hidden">
          <div className="px-4 py-2 border-b border-th-border bg-th-surface-muted">
            <h2 className="text-xs font-semibold text-th-text-muted uppercase tracking-wide">Agent Status</h2>
          </div>
          <div className="flex flex-wrap gap-2 p-4">
            {agents.map(a => (
              <div
                key={a.id}
                className="flex items-center gap-2 px-3 py-1.5 rounded-lg border border-th-border bg-th-surface-muted text-xs"
              >
                <span className="font-medium text-th-text">{a.name}</span>
                <StatusBadge status={a.status} />
                {a.tags.length > 0 && (
                  <span className="text-th-text-muted hidden sm:inline">({a.tags.join(', ')})</span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

    </div>
  )
}
