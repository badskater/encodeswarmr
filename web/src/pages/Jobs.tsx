import { useState, useEffect, useCallback } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import * as api from '../api/client'
import type { Job } from '../types'
import StatusBadge from '../components/StatusBadge'
import ProgressBar from '../components/ProgressBar'
import { useAutoRefresh } from '../hooks/useAutoRefresh'

function basename(p: string) {
  return p.split(/[\\/]/).pop() ?? p
}

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

function isActiveJob(status: string) {
  return status !== 'completed' && status !== 'cancelled'
}

const STATUSES = ['', 'queued', 'assigned', 'running', 'completed', 'failed', 'cancelled']

export default function Jobs() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [status, setStatus] = useState('')
  const [search, setSearch] = useState('')
  const [nextCursor, setNextCursor] = useState<string | undefined>()
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [bulkCancelling, setBulkCancelling] = useState(false)
  const [showExport, setShowExport] = useState(false)
  const [exportFormat, setExportFormat] = useState<'csv' | 'json'>('csv')
  const [exportFrom, setExportFrom] = useState('')
  const [exportTo, setExportTo] = useState('')
  const navigate = useNavigate()

  const load = useCallback(async () => {
    try {
      const result = await api.listJobsPaged({ status: status || undefined, search: search || undefined })
      setJobs(result.items)
      setNextCursor(result.nextCursor)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [status, search])

  useEffect(() => { setLoading(true); load() }, [load])
  useAutoRefresh(load)

  const handleLoadMore = async () => {
    if (!nextCursor) return
    setLoadingMore(true)
    try {
      const result = await api.listJobsPaged({ status: status || undefined, search: search || undefined, cursor: nextCursor })
      setJobs(prev => [...prev, ...result.items])
      setNextCursor(result.nextCursor)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load more')
    } finally {
      setLoadingMore(false)
    }
  }

  const toggleSelect = (id: string) => {
    setSelected(s => {
      const n = new Set(s)
      if (n.has(id)) n.delete(id); else n.add(id)
      return n
    })
  }

  const activeSelected = jobs.filter(j => selected.has(j.id) && isActiveJob(j.status))

  const allChecked = jobs.length > 0 && jobs.every(j => selected.has(j.id))
  const someChecked = jobs.some(j => selected.has(j.id))

  const toggleAll = () => {
    if (allChecked) {
      setSelected(new Set())
    } else {
      setSelected(new Set(jobs.map(j => j.id)))
    }
  }

  const handleBulkCancel = async () => {
    setBulkCancelling(true)
    setError('')
    try {
      for (const j of activeSelected) {
        await api.cancelJob(j.id)
      }
      setSelected(new Set())
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to bulk cancel jobs')
    } finally {
      setBulkCancelling(false)
    }
  }

  return (
    <div className="space-y-4">
      {/* Header: title + search/filter + action buttons */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-th-text">Jobs</h1>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:flex-1 sm:max-w-lg">
          <input
            type="text"
            placeholder="Search by ID or source path…"
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="flex-1 bg-th-input-bg border border-th-input-border rounded px-3 py-1.5 text-sm text-th-text focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <select
            value={status}
            onChange={e => setStatus(e.target.value)}
            className="bg-th-input-bg border border-th-input-border rounded px-3 py-1.5 text-sm text-th-text focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            {STATUSES.map(s => (
              <option key={s} value={s}>{s || 'All statuses'}</option>
            ))}
          </select>
        </div>
        <div className="flex items-center gap-2">
          {activeSelected.length > 0 && (
            <button
              onClick={handleBulkCancel}
              disabled={bulkCancelling}
              className="text-sm px-3 py-1.5 rounded font-medium disabled:opacity-50"
              style={{
                backgroundColor: 'var(--th-badge-error-bg)',
                color: 'var(--th-badge-error-text)',
              }}
            >
              {bulkCancelling ? 'Cancelling…' : `Cancel Selected (${activeSelected.length})`}
            </button>
          )}
          <button
            onClick={() => setShowExport(v => !v)}
            className="bg-th-surface-muted border border-th-border text-th-text-secondary px-3 py-1.5 rounded text-sm font-medium hover:bg-th-surface whitespace-nowrap"
          >
            Export
          </button>
          <Link
            to="/jobs/create-chain"
            className="border border-th-border text-th-text px-3 py-1.5 rounded text-sm font-medium hover:bg-th-surface-muted whitespace-nowrap"
          >
            New Chain
          </Link>
          <Link
            to="/jobs/create"
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 whitespace-nowrap"
          >
            New Job
          </Link>
        </div>
      </div>

      {showExport && (
        <div className="bg-th-surface rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-th-text-secondary">Export Job History</h2>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Format</label>
              <select
                value={exportFormat}
                onChange={e => setExportFormat(e.target.value as 'csv' | 'json')}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
              >
                <option value="csv">CSV</option>
                <option value="json">JSON</option>
              </select>
            </div>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Status filter</label>
              <select
                value={status}
                onChange={e => setStatus(e.target.value)}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
              >
                {STATUSES.map(s => <option key={s} value={s}>{s || 'All'}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">From (YYYY-MM-DD)</label>
              <input
                type="date"
                value={exportFrom}
                onChange={e => setExportFrom(e.target.value)}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
              />
            </div>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">To (YYYY-MM-DD)</label>
              <input
                type="date"
                value={exportTo}
                onChange={e => setExportTo(e.target.value)}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
              />
            </div>
          </div>
          <div className="flex gap-2">
            <a
              href={api.jobExportURL({ format: exportFormat, status: status || undefined, from: exportFrom || undefined, to: exportTo || undefined })}
              download
              className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
            >
              Download {exportFormat.toUpperCase()}
            </a>
            <button
              onClick={() => setShowExport(false)}
              className="px-3 py-1.5 rounded text-sm text-th-text-muted border border-th-border hover:bg-th-surface-muted"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {loading ? <p className="text-th-text-muted">Loading…</p> : (
        <>
          {/* Desktop table */}
          <div className="hidden sm:block bg-th-surface rounded-lg shadow overflow-hidden">
            <table className="min-w-full divide-y divide-th-border text-sm">
              <thead className="bg-th-surface-muted">
                <tr>
                  <th className="px-4 py-2 text-left">
                    <input
                      type="checkbox"
                      checked={allChecked}
                      ref={el => { if (el) el.indeterminate = someChecked && !allChecked }}
                      onChange={toggleAll}
                      className="rounded border-th-input-border"
                    />
                  </th>
                  {['ID', 'Source', 'Status', 'Progress', 'ETA', 'Priority', 'Created'].map(h => (
                    <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-th-border-subtle">
                {jobs.map(j => (
                  <tr key={j.id} className="hover:bg-th-surface-muted">
                    <td className="px-4 py-2" onClick={e => e.stopPropagation()}>
                      <input
                        type="checkbox"
                        checked={selected.has(j.id)}
                        onChange={() => toggleSelect(j.id)}
                        className="rounded border-th-input-border"
                      />
                    </td>
                    <td className="px-4 py-2 font-mono text-blue-600 cursor-pointer" onClick={() => navigate(`/jobs/${j.id}`)}>{j.id.slice(0, 8)}</td>
                    <td className="px-4 py-2 max-w-xs truncate text-th-text-secondary cursor-pointer" onClick={() => navigate(`/jobs/${j.id}`)}>{basename(j.source_path)}</td>
                    <td className="px-4 py-2 cursor-pointer" onClick={() => navigate(`/jobs/${j.id}`)}><StatusBadge status={j.status} /></td>
                    <td className="px-4 py-2 w-36 cursor-pointer" onClick={() => navigate(`/jobs/${j.id}`)}>
                      <ProgressBar value={j.tasks_completed} max={j.tasks_total} />
                      <span className="text-xs text-th-text-subtle">{j.tasks_completed}/{j.tasks_total}</span>
                    </td>
                    <td className="px-4 py-2 text-th-text-muted text-xs whitespace-nowrap cursor-pointer" onClick={() => navigate(`/jobs/${j.id}`)}>
                      {j.eta_human ? `~${j.eta_human}` : '—'}
                    </td>
                    <td className="px-4 py-2 text-th-text-secondary cursor-pointer" onClick={() => navigate(`/jobs/${j.id}`)}>{j.priority}</td>
                    <td className="px-4 py-2 text-th-text-muted whitespace-nowrap cursor-pointer" onClick={() => navigate(`/jobs/${j.id}`)}>{fmtDate(j.created_at)}</td>
                  </tr>
                ))}
                {jobs.length === 0 && (
                  <tr><td colSpan={8} className="px-4 py-4 text-center text-th-text-subtle">No jobs found</td></tr>
                )}
              </tbody>
            </table>
          </div>

          {/* Mobile card list */}
          <div className="sm:hidden bg-th-surface rounded-lg shadow divide-y divide-th-border-subtle">
            {jobs.map(j => (
              <div key={j.id} className="flex items-start gap-3 px-4 py-3 hover:bg-th-surface-muted">
                <input
                  type="checkbox"
                  checked={selected.has(j.id)}
                  onChange={() => toggleSelect(j.id)}
                  className="mt-0.5 rounded border-th-input-border shrink-0"
                />
                <div className="flex-1 min-w-0 cursor-pointer" onClick={() => navigate(`/jobs/${j.id}`)}>
                  <div className="flex items-center justify-between gap-2 mb-1">
                    <span className="font-mono text-xs text-blue-600">{j.id.slice(0, 8)}</span>
                    <StatusBadge status={j.status} />
                  </div>
                  <p className="text-sm text-th-text-secondary truncate">{basename(j.source_path)}</p>
                  <div className="mt-1.5">
                    <ProgressBar value={j.tasks_completed} max={j.tasks_total} />
                    <div className="flex justify-between mt-0.5 text-xs text-th-text-subtle">
                      <span>{j.tasks_completed}/{j.tasks_total} tasks · p{j.priority}</span>
                      <span>{fmtDate(j.created_at)}</span>
                    </div>
                  </div>
                </div>
              </div>
            ))}
            {jobs.length === 0 && (
              <p className="px-4 py-4 text-center text-th-text-subtle text-sm">No jobs found</p>
            )}
          </div>

          {nextCursor && (
            <div className="text-center">
              <button
                onClick={handleLoadMore}
                disabled={loadingMore}
                className="px-4 py-2 text-sm text-th-text-secondary border border-th-border rounded hover:bg-th-surface-muted disabled:opacity-50"
              >
                {loadingMore ? 'Loading…' : 'Load more'}
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
