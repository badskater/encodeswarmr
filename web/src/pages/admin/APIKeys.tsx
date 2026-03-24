import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'

export default function APIKeys() {
  const [keys, setKeys] = useState<api.APIKeyInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editingRateLimit, setEditingRateLimit] = useState<{ id: string; value: string } | null>(null)
  const [savingRateLimit, setSavingRateLimit] = useState(false)

  const load = useCallback(async () => {
    try {
      const data = await api.listAPIKeys()
      setKeys(data)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load API keys')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleSaveRateLimit = async () => {
    if (!editingRateLimit) return
    const val = parseInt(editingRateLimit.value, 10)
    if (isNaN(val) || val < 0) {
      setError('Rate limit must be a non-negative integer (0 = use global default).')
      return
    }
    setSavingRateLimit(true)
    setError('')
    try {
      await api.updateAPIKeyRateLimit(editingRateLimit.id, val)
      setEditingRateLimit(null)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to update rate limit')
    } finally {
      setSavingRateLimit(false)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-6 max-w-4xl">
      <div>
        <h1 className="text-2xl font-bold text-th-text">API Keys</h1>
        <p className="text-sm text-th-text-muted mt-0.5">
          Manage API keys and configure per-key rate limits.
        </p>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {keys.length === 0 ? (
        <p className="text-sm text-th-text-muted">No API keys found.</p>
      ) : (
        <div className="bg-th-surface rounded-lg shadow overflow-hidden">
          <table className="min-w-full text-sm">
            <thead className="bg-th-surface-muted">
              <tr className="text-left text-th-text-muted border-b border-th-border">
                <th className="px-4 py-2 font-medium">Name</th>
                <th className="px-4 py-2 font-medium">ID</th>
                <th className="px-4 py-2 font-medium">Created</th>
                <th className="px-4 py-2 font-medium">Expires</th>
                <th className="px-4 py-2 font-medium">Last Used</th>
                <th className="px-4 py-2 font-medium">Rate Limit (req/min)</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-th-border">
              {keys.map(k => (
                <tr key={k.id} className="hover:bg-th-surface-muted">
                  <td className="px-4 py-2 font-medium text-th-text">{k.name}</td>
                  <td className="px-4 py-2 font-mono text-xs text-th-text-muted">{k.id.slice(0, 8)}…</td>
                  <td className="px-4 py-2 text-th-text-muted text-xs">
                    {new Date(k.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-2 text-th-text-muted text-xs">
                    {k.expires_at ? new Date(k.expires_at).toLocaleDateString() : '—'}
                  </td>
                  <td className="px-4 py-2 text-th-text-muted text-xs">
                    {k.last_used_at ? new Date(k.last_used_at).toLocaleString() : '—'}
                  </td>
                  <td className="px-4 py-2">
                    {editingRateLimit?.id === k.id ? (
                      <div className="flex items-center gap-2">
                        <input
                          type="number"
                          min={0}
                          value={editingRateLimit.value}
                          onChange={e => setEditingRateLimit({ id: k.id, value: e.target.value })}
                          className="w-24 rounded border border-th-border bg-th-input px-2 py-1 text-sm text-th-text focus:outline-none focus:ring-1 focus:ring-blue-500"
                        />
                        <button
                          onClick={handleSaveRateLimit}
                          disabled={savingRateLimit}
                          className="text-xs text-blue-600 hover:underline disabled:opacity-50"
                        >
                          {savingRateLimit ? 'Saving…' : 'Save'}
                        </button>
                        <button
                          onClick={() => setEditingRateLimit(null)}
                          className="text-xs text-th-text-muted hover:underline"
                        >
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <div className="flex items-center gap-2">
                        <span className="text-th-text">
                          {k.rate_limit === 0 ? (
                            <span className="text-th-text-muted italic">global default</span>
                          ) : (
                            k.rate_limit
                          )}
                        </span>
                        <button
                          onClick={() => setEditingRateLimit({ id: k.id, value: String(k.rate_limit) })}
                          className="text-xs text-blue-600 hover:underline"
                        >
                          Edit
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="bg-th-surface rounded-lg shadow p-4 text-xs text-th-text-muted space-y-1">
        <p className="font-medium text-th-text text-sm">About rate limits</p>
        <p>
          Set to <span className="font-mono">0</span> to use the global default (200 req/s per IP).
          A positive value caps requests per minute for that key, regardless of IP.
          The stricter of the per-key and per-IP limit applies.
        </p>
      </div>
    </div>
  )
}
