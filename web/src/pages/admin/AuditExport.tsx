import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { AuditStats, AuditActionStat } from '../../types'

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

export default function AuditExport() {
  const [stats, setStats] = useState<AuditStats | null>(null)
  const [statsLoading, setStatsLoading] = useState(true)
  const [error, setError] = useState('')

  // Export filter state
  const [format, setFormat] = useState<'csv' | 'json'>('csv')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [userId, setUserId] = useState('')
  const [action, setAction] = useState('')

  const loadStats = useCallback(async () => {
    try {
      const s = await api.getAuditLogStats()
      setStats(s)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load stats')
    } finally {
      setStatsLoading(false)
    }
  }, [])

  useEffect(() => { loadStats() }, [loadStats])

  const handleDownload = () => {
    const url = api.auditLogExportURL({
      format,
      from: from || undefined,
      to: to || undefined,
      user_id: userId || undefined,
      action: action || undefined,
    })
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-th-text">Audit Log Export</h1>
      {error && <p className="text-red-600 text-sm">{error}</p>}

      {/* Stats summary */}
      <div className="bg-th-surface rounded-lg shadow p-4 space-y-3">
        <h2 className="text-sm font-semibold text-th-text-secondary uppercase tracking-wide">Summary</h2>
        {statsLoading ? (
          <p className="text-th-text-muted text-sm">Loading…</p>
        ) : stats ? (
          <>
            <p className="text-th-text text-sm">
              <span className="font-medium">{stats.total.toLocaleString()}</span> total audit entries
            </p>
            {stats.per_action && stats.per_action.length > 0 && (
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-th-border text-sm">
                  <thead className="bg-th-surface-muted">
                    <tr>
                      <th className="px-3 py-1.5 text-left text-xs font-medium text-th-text-muted uppercase">Action</th>
                      <th className="px-3 py-1.5 text-right text-xs font-medium text-th-text-muted uppercase">Count</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-th-border-subtle">
                    {stats.per_action.map((s: AuditActionStat) => (
                      <tr key={s.action} className="hover:bg-th-surface-muted">
                        <td className="px-3 py-1.5 text-th-text font-mono text-xs">{s.action}</td>
                        <td className="px-3 py-1.5 text-right text-th-text-secondary">{s.count.toLocaleString()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        ) : null}
      </div>

      {/* Export filters */}
      <div className="bg-th-surface rounded-lg shadow p-4 space-y-4">
        <h2 className="text-sm font-semibold text-th-text-secondary uppercase tracking-wide">Export Filters</h2>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label className="block text-xs text-th-text-muted mb-1">From (date or datetime)</label>
            <input
              type="text"
              placeholder="e.g. 2024-01-01 or 2024-01-01T00:00:00Z"
              value={from}
              onChange={e => setFrom(e.target.value)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
            />
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">To (date or datetime)</label>
            <input
              type="text"
              placeholder="e.g. 2024-12-31 or 2024-12-31T23:59:59Z"
              value={to}
              onChange={e => setTo(e.target.value)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
            />
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">User ID (optional)</label>
            <input
              type="text"
              placeholder="Filter by user UUID"
              value={userId}
              onChange={e => setUserId(e.target.value)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
            />
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Action type (optional)</label>
            <input
              type="text"
              placeholder={`e.g. job.created`}
              value={action}
              onChange={e => setAction(e.target.value)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
              list="action-suggestions"
            />
            {stats?.per_action && (
              <datalist id="action-suggestions">
                {stats.per_action.map((s: AuditActionStat) => (
                  <option key={s.action} value={s.action} />
                ))}
              </datalist>
            )}
          </div>
        </div>

        {/* Format toggle */}
        <div>
          <p className="text-xs text-th-text-muted mb-2">Format</p>
          <div className="flex gap-2">
            {(['csv', 'json'] as const).map(f => (
              <button
                key={f}
                onClick={() => setFormat(f)}
                className={`text-sm px-3 py-1.5 rounded font-medium border transition-colors ${
                  format === f
                    ? 'border-blue-500 bg-blue-600 text-white'
                    : 'border-th-input-border text-th-text-muted hover:bg-th-surface-muted'
                }`}
              >
                {f.toUpperCase()}
              </button>
            ))}
          </div>
        </div>

        <button
          onClick={handleDownload}
          className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700"
        >
          Download {format.toUpperCase()}
        </button>
      </div>
    </div>
  )
}
