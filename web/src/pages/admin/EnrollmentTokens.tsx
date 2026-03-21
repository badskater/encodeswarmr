import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { EnrollmentToken } from '../../types'

function fmtDate(s: string | null) {
  return s ? new Date(s).toLocaleString() : '—'
}

export default function EnrollmentTokens() {
  const [tokens, setTokens] = useState<EnrollmentToken[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [expiresAt, setExpiresAt] = useState('')
  const [saving, setSaving] = useState(false)
  const [newToken, setNewToken] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const t = await api.listEnrollmentTokens()
      setTokens(t)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    try {
      const body = expiresAt ? { expires_at: new Date(expiresAt).toISOString() } : undefined
      const created = await api.createEnrollmentToken(body)
      setNewToken(created.token)
      setShowForm(false)
      setExpiresAt('')
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create token')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this enrollment token? Agents using it will not be affected.')) return
    try {
      await api.deleteEnrollmentToken(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Enrollment Tokens</h1>
        <button
          onClick={() => { setShowForm(!showForm); setNewToken(null) }}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          {showForm ? 'Cancel' : 'Create Token'}
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {newToken && (
        <div className="bg-green-50 border border-green-300 rounded-lg p-4 space-y-2">
          <p className="text-sm font-semibold text-green-800">Token created — copy it now, it will not be shown again.</p>
          <code className="block font-mono text-sm text-green-900 break-all bg-green-100 rounded px-3 py-2 select-all">
            {newToken}
          </code>
          <button
            onClick={() => setNewToken(null)}
            className="text-xs text-green-700 hover:underline"
          >
            Dismiss
          </button>
        </div>
      )}

      {showForm && (
        <form onSubmit={handleCreate} className="bg-th-surface rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-th-text-secondary">New Enrollment Token</h2>
          <div className="max-w-xs">
            <label className="block text-xs text-th-text-muted mb-1">Expires At (optional)</label>
            <input
              type="datetime-local"
              value={expiresAt}
              onChange={e => setExpiresAt(e.target.value)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
            />
          </div>
          <button
            type="submit"
            disabled={saving}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
          >
            {saving ? 'Creating…' : 'Create Token'}
          </button>
        </form>
      )}

      <div className="bg-th-surface rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-th-border text-sm">
          <thead className="bg-th-surface-muted">
            <tr>
              {['Token', 'Expires At', 'Used By', 'Used At', 'Created', 'Actions'].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-th-border-subtle">
            {tokens.map(t => (
              <tr key={t.id} className="hover:bg-th-surface-muted">
                <td className="px-4 py-2 font-mono text-xs text-th-text break-all max-w-xs">{t.token}</td>
                <td className="px-4 py-2 text-th-text-secondary whitespace-nowrap">{fmtDate(t.expires_at)}</td>
                <td className="px-4 py-2 text-th-text-secondary font-mono text-xs">{t.used_by ?? '—'}</td>
                <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(t.used_at)}</td>
                <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(t.created_at)}</td>
                <td className="px-4 py-2">
                  <button
                    onClick={() => handleDelete(t.id)}
                    className="text-xs text-red-600 hover:underline"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
            {tokens.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-4 text-center text-th-text-subtle">No enrollment tokens</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
