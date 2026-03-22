import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import * as api from '../api/client'
import type { Job, Task, ComparisonResponse } from '../types'
import StatusBadge from '../components/StatusBadge'
import ProgressBar from '../components/ProgressBar'
import ComparisonCard from '../components/ComparisonCard'
import { useAutoRefresh } from '../hooks/useAutoRefresh'

function fmtBytes(n: number) {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + ' GB'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + ' MB'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + ' KB'
  return n + ' B'
}

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex py-2 border-b border-th-border-subtle last:border-0">
      <span className="w-40 text-sm text-th-text-muted shrink-0">{label}</span>
      <span className="text-sm text-th-text">{value}</span>
    </div>
  )
}

export default function JobDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [job, setJob] = useState<Job | null>(null)
  const [tasks, setTasks] = useState<Task[]>([])
  const [comparison, setComparison] = useState<ComparisonResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')

  const load = useCallback(async () => {
    if (!id) return
    try {
      const { job: j, tasks: t } = await api.getJob(id)
      setJob(j)
      setTasks(t)
      setError('')
      // Fetch comparison data for completed encode jobs.
      if (j.status === 'completed' && j.job_type === 'encode') {
        try {
          const cmp = await api.getJobComparison(id)
          setComparison(cmp)
        } catch {
          // Non-fatal — comparison data may not be available yet.
        }
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { load() }, [load])
  useAutoRefresh(load)

  const handleCancel = async () => {
    if (!id) return
    try {
      await api.cancelJob(id)
      load()
    } catch (e: unknown) {
      setActionError(e instanceof Error ? e.message : 'Failed to cancel')
    }
  }

  const handleRetry = async () => {
    if (!id) return
    try {
      await api.retryJob(id)
      load()
    } catch (e: unknown) {
      setActionError(e instanceof Error ? e.message : 'Failed to retry')
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>
  if (error) return <p className="text-red-600">{error}</p>
  if (!job) return <p className="text-th-text-muted">Job not found</p>

  const canCancel = ['queued', 'assigned', 'running'].includes(job.status)
  const canRetry = job.status === 'failed'

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <button onClick={() => navigate('/jobs')} className="text-blue-600 hover:underline text-sm">← Jobs</button>
        <h1 className="text-xl font-bold text-th-text font-mono">{job.id.slice(0, 8)}…</h1>
        <StatusBadge status={job.status} />
      </div>

      {actionError && <p className="text-red-600 text-sm">{actionError}</p>}

      <div className="bg-th-surface rounded-lg shadow p-4">
        <h2 className="text-sm font-semibold text-th-text-secondary mb-2">Job Details</h2>
        <Row label="ID" value={<span className="font-mono text-xs">{job.id}</span>} />
        <Row label="Type" value={<span className="capitalize">{job.job_type}</span>} />
        <Row label="Source" value={<span className="font-mono text-xs break-all">{job.source_path}</span>} />
        <Row label="Priority" value={job.priority} />
        <Row label="Created" value={fmtDate(job.created_at)} />
        <Row label="Updated" value={fmtDate(job.updated_at)} />
      </div>

      <div className="bg-th-surface rounded-lg shadow p-4">
        <h2 className="text-sm font-semibold text-th-text-secondary mb-3">Progress</h2>
        <ProgressBar value={job.tasks_completed} max={job.tasks_total} className="mb-2" />
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-5 text-sm mt-3">
          {[
            { label: 'Total', value: job.tasks_total },
            { label: 'Completed', value: job.tasks_completed },
            { label: 'Running', value: job.tasks_running },
            { label: 'Pending', value: job.tasks_pending },
            { label: 'Failed', value: job.tasks_failed },
          ].map(s => (
            <div key={s.label} className="bg-th-surface-muted rounded p-2 text-center">
              <p className="text-xs text-th-text-muted">{s.label}</p>
              <p className="font-bold text-th-text">{s.value}</p>
            </div>
          ))}
        </div>
        {job.eta_human && (
          <p className="mt-3 text-sm text-th-text-muted">
            ETA: <span className="font-medium text-th-text">~{job.eta_human} remaining</span>
          </p>
        )}
      </div>

      <div className="flex gap-3">
        {canCancel && (
          <button
            onClick={handleCancel}
            className="bg-red-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-red-700"
          >
            Cancel Job
          </button>
        )}
        {canRetry && (
          <button
            onClick={handleRetry}
            className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700"
          >
            Retry Failed Tasks
          </button>
        )}
      </div>

      {job.tasks_total > 1 && job.job_type === 'encode' && (
        <div className="bg-th-surface-muted border border-th-border rounded p-3 text-xs space-y-2">
          <p className="font-medium text-th-text-secondary">Multi-chunk encode — merge required after completion</p>
          <p className="text-th-text-muted">After all chunks complete, concatenate with ffmpeg:</p>
          <pre className="bg-th-bg rounded p-2 font-mono text-th-text overflow-x-auto">
            {`# Create file list\n` +
             Array.from({ length: job.tasks_total }, (_, i) =>
               `echo "file 'chunk_${String(i).padStart(4, '0')}.mkv'" >> filelist.txt`
             ).join('\n') +
             `\n\n# Concatenate\nffmpeg -f concat -safe 0 -i filelist.txt -c copy output.mkv`}
          </pre>
          <p className="text-th-text-subtle">Adjust filenames to match your output extension and paths.</p>
        </div>
      )}

      {comparison && <ComparisonCard data={comparison} />}

      <div className="bg-th-surface rounded-lg shadow overflow-hidden">
        <div className="px-4 py-3 border-b border-th-border">
          <h2 className="text-sm font-semibold text-th-text-secondary">
            Tasks ({tasks.length})
          </h2>
        </div>

        {tasks.length === 0 ? (
          <p className="px-4 py-6 text-sm text-th-text-muted">
            {job.tasks_total === 0
              ? 'No tasks created yet — the engine will expand this job shortly.'
              : 'No task data available.'}
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-th-border bg-th-surface-muted">
                  <th className="text-left px-4 py-2 text-xs text-th-text-muted font-medium">#</th>
                  <th className="text-left px-4 py-2 text-xs text-th-text-muted font-medium">Status</th>
                  <th className="text-left px-4 py-2 text-xs text-th-text-muted font-medium">Agent</th>
                  <th className="text-right px-4 py-2 text-xs text-th-text-muted font-medium">Frames</th>
                  <th className="text-right px-4 py-2 text-xs text-th-text-muted font-medium">Size</th>
                  <th className="text-left px-4 py-2 text-xs text-th-text-muted font-medium">Error</th>
                  <th className="px-4 py-2"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-th-border-subtle">
                {tasks.map(task => (
                  <tr key={task.id} className="hover:bg-th-surface-muted transition-colors">
                    <td className="px-4 py-2 text-th-text-muted">{task.chunk_index}</td>
                    <td className="px-4 py-2"><StatusBadge status={task.status} /></td>
                    <td className="px-4 py-2 font-mono text-xs text-th-text-muted">
                      {task.agent_id ? task.agent_id.slice(0, 8) + '…' : '—'}
                    </td>
                    <td className="px-4 py-2 text-right text-th-text-muted">
                      {task.frames_encoded != null && task.frames_encoded > 0
                        ? task.frames_encoded.toLocaleString()
                        : '—'}
                    </td>
                    <td className="px-4 py-2 text-right text-th-text-muted">
                      {task.output_size != null && task.output_size > 0
                        ? fmtBytes(task.output_size)
                        : '—'}
                    </td>
                    <td className="px-4 py-2 text-red-600 text-xs truncate max-w-[180px]">
                      {task.error_msg ?? ''}
                    </td>
                    <td className="px-4 py-2 text-right">
                      <Link
                        to={`/tasks/${task.id}`}
                        className="text-blue-600 hover:underline text-xs whitespace-nowrap"
                      >
                        View logs →
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
