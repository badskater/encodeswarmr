import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { APIKey } from '../../types'

function fmtDate(s: string | null) {
  return s ? new Date(s).toLocaleString() : '—'
}

export default function APIKeys() {
  const [keys, setKeys] = useState<APIKey[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ name: '', rate_limit: 200 })
  const [saving, setSaving] = useState(false)
  const [newKey, setNewKey] = useState<string | null>(null)
  const [deleting, setDeleting] = useState<string | null>(null)
  const [rateLimitEdits, setRateLimitEdits] = useState<Record<string, number>>({})
  const [savingRateLimit, setSavingRateLimit] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const k = await api.listAPIKeys()
      setKeys(k)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load API keys')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setNewKey(null)
    try {
      const result = await api.createAPIKey(form)
      setNewKey(result.key)
      setShowForm(false)
      setForm({ name: '', rate_limit: 200 })
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create API key')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete API key "${name}"? This cannot be undone.`)) return
    setDeleting(id)
    try {
      await api.deleteAPIKey(id)
      if (newKey) setNewKey(null)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete API key')
    } finally {
      setDeleting(null)
    }
  }

  const handleRateLimitSave = async (id: string) => {
    const rateLimit = rateLimitEdits[id]
    if (rateLimit === undefined) return
    setSavingRateLimit(id)
    try {
      await api.updateAPIKeyRateLimit(id, rateLimit)
      setRateLimitEdits(r => { const c = { ...r }; delete c[id]; return c })
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to update rate limit')
    } finally {
      setSavingRateLimit(null)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">API Keys</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          {showForm ? 'Cancel' : 'New API Key'}
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {/* New key revealed after creation */}
      {newKey && (
        <div className="bg-yellow-50 border border-yellow-300 rounded-lg p-4 space-y-2">
          <p className="text-sm font-semibold text-yellow-800">Copy this key — it will not be shown again.</p>
          <pre className="text-xs font-mono bg-yellow-100 rounded px-3 py-2 break-all select-all text-yellow-900">{newKey}</pre>
          <button
            onClick={() => setNewKey(null)}
            className="text-xs text-yellow-700 hover:underline"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Create form */}
      {showForm && (
        <form onSubmit={handleCreate} className="bg-th-surface rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-th-text-secondary">New API Key</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Name</label>
              <input
                value={form.name}
                onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                placeholder="e.g. CI pipeline"
                required
              />
            </div>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Rate Limit (req/s)</label>
              <input
                type="number"
                min={1}
                max={10000}
                value={form.rate_limit}
                onChange={e => setForm(f => ({ ...f, rate_limit: parseInt(e.target.value, 10) || 200 }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
              />
              <p className="text-xs text-th-text-muted mt-0.5">Default: 200, max: 10 000</p>
            </div>
          </div>
          <button
            type="submit"
            disabled={saving}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
          >
            {saving ? 'Creating…' : 'Create Key'}
          </button>
        </form>
      )}

      <div className="bg-th-surface rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-th-border text-sm">
          <thead className="bg-th-surface-muted">
            <tr>
              {['Name', 'Rate Limit (req/s)', 'Created', 'Last Used', 'Expires', ''].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-th-border-subtle">
            {keys.map(k => {
              const editedRateLimit = rateLimitEdits[k.id]
              const displayRateLimit = editedRateLimit ?? k.rate_limit
              const rateChanged = editedRateLimit !== undefined && editedRateLimit !== k.rate_limit
              return (
                <tr key={k.id} className="hover:bg-th-surface-muted">
                  <td className="px-4 py-2 font-medium text-th-text">{k.name}</td>
                  <td className="px-4 py-2">
                    <div className="flex items-center gap-2">
                      <input
                        type="number"
                        min={1}
                        max={10000}
                        value={displayRateLimit}
                        onChange={e => setRateLimitEdits(r => ({
                          ...r,
                          [k.id]: parseInt(e.target.value, 10) || k.rate_limit,
                        }))}
                        className="w-20 bg-th-input-bg border border-th-input-border rounded px-2 py-0.5 text-xs text-th-text"
                      />
                      {rateChanged && (
                        <button
                          onClick={() => handleRateLimitSave(k.id)}
                          disabled={savingRateLimit === k.id}
                          className="text-xs px-2 py-0.5 rounded disabled:opacity-50"
                          style={{
                            backgroundColor: 'var(--th-badge-running-bg)',
                            color: 'var(--th-badge-running-text)',
                          }}
                        >
                          {savingRateLimit === k.id ? 'Saving…' : 'Save'}
                        </button>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(k.created_at)}</td>
                  <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(k.last_used_at)}</td>
                  <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(k.expires_at)}</td>
                  <td className="px-4 py-2">
                    <button
                      onClick={() => handleDelete(k.id, k.name)}
                      disabled={deleting === k.id}
                      className="text-xs text-red-600 hover:underline disabled:opacity-50"
                    >
                      {deleting === k.id ? 'Deleting…' : 'Delete'}
                    </button>
                  </td>
                </tr>
              )
            })}
            {keys.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-4 text-center text-th-text-subtle">No API keys</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
