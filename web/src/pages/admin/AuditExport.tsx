import { useState } from 'react'
import * as api from '../../api/client'

export default function AuditExport() {
  const [format, setFormat] = useState<'csv' | 'json'>('csv')
  const [userID, setUserID] = useState('')
  const PAGE_SIZE = 50
  const [activityPage, setActivityPage] = useState(1)
  const [activityData, setActivityData] = useState<api.AuditEntry[]>([])
  const [activityLoading, setActivityLoading] = useState(false)
  const [activityError, setActivityError] = useState('')

  const handleExport = () => {
    const url = api.auditLogExportURL({ format })
    window.location.href = url
  }

  const handleLoadActivity = async (page: number) => {
    if (!userID.trim()) return
    setActivityLoading(true)
    setActivityError('')
    try {
      const offset = (page - 1) * PAGE_SIZE
      const result = await api.getUserActivity(userID.trim(), PAGE_SIZE, offset)
      setActivityData(result.items)
    } catch (e: unknown) {
      setActivityError(e instanceof Error ? e.message : 'Failed to load activity')
    } finally {
      setActivityLoading(false)
    }
  }

  const handleLoadClick = () => handleLoadActivity(activityPage)

  return (
    <div className="space-y-6 max-w-4xl">
      <div>
        <h1 className="text-2xl font-bold text-th-text">Audit Log Export</h1>
        <p className="text-sm text-th-text-muted mt-0.5">
          Export the full audit log or view activity for a specific user.
        </p>
      </div>

      {/* Export section */}
      <section className="bg-th-surface rounded-lg shadow p-5 space-y-4">
        <h2 className="text-sm font-semibold text-th-text">Export Full Audit Log</h2>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <label className="text-sm text-th-text font-medium">Format:</label>
            <select
              value={format}
              onChange={e => setFormat(e.target.value as 'csv' | 'json')}
              className="rounded border border-th-border bg-th-input px-3 py-1.5 text-sm text-th-text focus:outline-none focus:ring-1 focus:ring-blue-500"
            >
              <option value="csv">CSV</option>
              <option value="json">JSON</option>
            </select>
          </div>
          <button
            onClick={handleExport}
            className="rounded bg-blue-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-blue-700"
          >
            Download
          </button>
        </div>
        <p className="text-xs text-th-text-muted">
          Downloads the complete audit log. CSV is suitable for spreadsheets; JSON preserves full data types.
        </p>
      </section>

      {/* User activity section */}
      <section className="bg-th-surface rounded-lg shadow p-5 space-y-4">
        <h2 className="text-sm font-semibold text-th-text">User Activity</h2>
        <div className="flex gap-2">
          <input
            type="text"
            value={userID}
            onChange={e => setUserID(e.target.value)}
            placeholder="User ID (UUID)"
            className="flex-1 rounded border border-th-border bg-th-input px-3 py-1.5 text-sm text-th-text focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
          />
          <button
            onClick={handleLoadClick}
            disabled={activityLoading || !userID.trim()}
            className="rounded bg-th-surface-muted border border-th-border px-3 py-1.5 text-sm text-th-text hover:bg-th-surface disabled:opacity-50"
          >
            {activityLoading ? 'Loading…' : 'Load'}
          </button>
        </div>

        {activityError && <p className="text-red-600 text-sm">{activityError}</p>}

        {activityData.length > 0 && (
          <>
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead>
                  <tr className="text-left text-th-text-muted border-b border-th-border">
                    <th className="pb-2 pr-4 font-medium">Action</th>
                    <th className="pb-2 pr-4 font-medium">Resource</th>
                    <th className="pb-2 pr-4 font-medium">Resource ID</th>
                    <th className="pb-2 font-medium">Time</th>
                  </tr>
                </thead>
                <tbody>
                  {activityData.map(entry => (
                    <tr key={entry.id} className="border-b border-th-border/50">
                      <td className="py-1.5 pr-4 text-th-text font-mono text-xs">{entry.action}</td>
                      <td className="py-1.5 pr-4 text-th-text-muted">{entry.resource}</td>
                      <td className="py-1.5 pr-4 text-th-text-muted font-mono text-xs">
                        {entry.resource_id ? entry.resource_id.slice(0, 8) + '…' : '—'}
                      </td>
                      <td className="py-1.5 text-th-text-muted text-xs">
                        {new Date(entry.logged_at).toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => {
                  const newPage = Math.max(1, activityPage - 1)
                  setActivityPage(newPage)
                  handleLoadActivity(newPage)
                }}
                disabled={activityPage <= 1 || activityLoading}
                className="rounded border border-th-border px-3 py-1 text-sm text-th-text hover:bg-th-surface-muted disabled:opacity-50"
              >
                Previous
              </button>
              <span className="text-sm text-th-text-muted">Page {activityPage}</span>
              <button
                onClick={() => {
                  const newPage = activityPage + 1
                  setActivityPage(newPage)
                  handleLoadActivity(newPage)
                }}
                disabled={activityLoading || activityData.length < PAGE_SIZE}
                className="rounded border border-th-border px-3 py-1 text-sm text-th-text hover:bg-th-surface-muted disabled:opacity-50"
              >
                Next
              </button>
            </div>
          </>
        )}
      </section>
    </div>
  )
}
