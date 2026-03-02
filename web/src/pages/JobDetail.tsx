import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import type { Job } from '../types'
import StatusBadge from '../components/StatusBadge'
import ProgressBar from '../components/ProgressBar'
import { useAutoRefresh } from '../hooks/useAutoRefresh'

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex py-2 border-b border-gray-100 last:border-0">
      <span className="w-40 text-sm text-gray-500 shrink-0">{label}</span>
      <span className="text-sm text-gray-900">{value}</span>
    </div>
  )
}

export default function JobDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [job, setJob] = useState<Job | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [actionError, setActionError] = useState('')

  const load = useCallback(async () => {
    if (!id) return
    try {
      const j = await api.getJob(id)
      setJob(j)
      setError('')
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

  if (loading) return <p className="text-gray-500">Loading…</p>
  if (error) return <p className="text-red-600">{error}</p>
  if (!job) return <p className="text-gray-500">Job not found</p>

  const canCancel = ['queued', 'assigned', 'running'].includes(job.status)
  const canRetry = job.status === 'failed'

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <button onClick={() => navigate('/jobs')} className="text-blue-600 hover:underline text-sm">← Jobs</button>
        <h1 className="text-xl font-bold text-gray-900 font-mono">{job.id.slice(0, 8)}…</h1>
        <StatusBadge status={job.status} />
      </div>

      {actionError && <p className="text-red-600 text-sm">{actionError}</p>}

      <div className="bg-white rounded-lg shadow p-4">
        <h2 className="text-sm font-semibold text-gray-700 mb-2">Job Details</h2>
        <Row label="ID" value={<span className="font-mono text-xs">{job.id}</span>} />
        <Row label="Source" value={<span className="font-mono text-xs break-all">{job.source_path}</span>} />
        <Row label="Priority" value={job.priority} />
        <Row label="Created" value={fmtDate(job.created_at)} />
        <Row label="Updated" value={fmtDate(job.updated_at)} />
      </div>

      <div className="bg-white rounded-lg shadow p-4">
        <h2 className="text-sm font-semibold text-gray-700 mb-3">Progress</h2>
        <ProgressBar value={job.tasks_completed} max={job.tasks_total} className="mb-2" />
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-5 text-sm mt-3">
          {[
            { label: 'Total', value: job.tasks_total },
            { label: 'Completed', value: job.tasks_completed },
            { label: 'Running', value: job.tasks_running },
            { label: 'Pending', value: job.tasks_pending },
            { label: 'Failed', value: job.tasks_failed },
          ].map(s => (
            <div key={s.label} className="bg-gray-50 rounded p-2 text-center">
              <p className="text-xs text-gray-500">{s.label}</p>
              <p className="font-bold text-gray-900">{s.value}</p>
            </div>
          ))}
        </div>
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

      {job.tasks_total > 0 && (
        <div className="bg-white rounded-lg shadow p-4">
          <h2 className="text-sm font-semibold text-gray-700 mb-2">Tasks</h2>
          <p className="text-sm text-gray-500">
            This job has {job.tasks_total} task{job.tasks_total !== 1 ? 's' : ''}.
            Navigate to <span className="font-mono text-xs">/tasks/&#123;task_id&#125;</span> to view individual task logs and details.
          </p>
        </div>
      )}
    </div>
  )
}
