import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { PathMapping } from '../../types'

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

export default function PathMappings() {
  const [mappings, setMappings] = useState<PathMapping[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ name: '', windows_prefix: '', linux_prefix: '', enabled: true })
  const [saving, setSaving] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [editValues, setEditValues] = useState<Record<string, { name: string; windows_prefix: string; linux_prefix: string; enabled: boolean }>>({})

  const load = useCallback(async () => {
    try {
      const m = await api.listPathMappings()
      setMappings(m)
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
      await api.createPathMapping(form)
      setShowForm(false)
      setForm({ name: '', windows_prefix: '', linux_prefix: '', enabled: true })
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create mapping')
    } finally {
      setSaving(false)
    }
  }

  const handleSaveEdit = async (m: PathMapping) => {
    const ed = editValues[m.id]
    if (!ed) return
    try {
      await api.updatePathMapping(m.id, ed)
      setEditId(null)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to update')
    }
  }

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete path mapping "${name}"?`)) return
    try {
      await api.deletePathMapping(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  const startEdit = (m: PathMapping) => {
    setEditId(m.id)
    setEditValues(ev => ({
      ...ev,
      [m.id]: { name: m.name, windows_prefix: m.windows_prefix, linux_prefix: m.linux_prefix, enabled: m.enabled },
    }))
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Path Mappings</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          {showForm ? 'Cancel' : 'Add Mapping'}
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {showForm && (
        <form onSubmit={handleCreate} className="bg-th-surface rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-th-text-secondary">New Path Mapping</h2>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Name</label>
              <input
                value={form.name}
                onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                required
              />
            </div>
            <div className="flex items-end gap-2">
              <label className="flex items-center gap-2 text-sm text-th-text-secondary cursor-pointer">
                <input
                  type="checkbox"
                  checked={form.enabled}
                  onChange={e => setForm(f => ({ ...f, enabled: e.target.checked }))}
                  className="rounded"
                />
                Enabled
              </label>
            </div>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Windows Prefix</label>
              <input
                value={form.windows_prefix}
                onChange={e => setForm(f => ({ ...f, windows_prefix: e.target.value }))}
                placeholder="\\server\share"
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm font-mono text-th-text"
                required
              />
            </div>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Linux Prefix</label>
              <input
                value={form.linux_prefix}
                onChange={e => setForm(f => ({ ...f, linux_prefix: e.target.value }))}
                placeholder="/mnt/share"
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm font-mono text-th-text"
                required
              />
            </div>
          </div>
          <button
            type="submit"
            disabled={saving}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
          >
            {saving ? 'Saving…' : 'Save Mapping'}
          </button>
        </form>
      )}

      <div className="bg-th-surface rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-th-border text-sm">
          <thead className="bg-th-surface-muted">
            <tr>
              {['Name', 'Windows Prefix', 'Linux Prefix', 'Enabled', 'Updated', 'Actions'].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-th-border-subtle">
            {mappings.map(m => (
              <tr key={m.id} className="hover:bg-th-surface-muted">
                <td className="px-4 py-2">
                  {editId === m.id ? (
                    <input
                      value={editValues[m.id]?.name ?? m.name}
                      onChange={e => setEditValues(ev => ({ ...ev, [m.id]: { ...ev[m.id], name: e.target.value } }))}
                      className="bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-xs text-th-text w-full"
                    />
                  ) : (
                    <span className="font-medium text-th-text">{m.name}</span>
                  )}
                </td>
                <td className="px-4 py-2">
                  {editId === m.id ? (
                    <input
                      value={editValues[m.id]?.windows_prefix ?? m.windows_prefix}
                      onChange={e => setEditValues(ev => ({ ...ev, [m.id]: { ...ev[m.id], windows_prefix: e.target.value } }))}
                      className="bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-xs font-mono text-th-text w-full"
                    />
                  ) : (
                    <span className="font-mono text-xs text-th-text-secondary">{m.windows_prefix}</span>
                  )}
                </td>
                <td className="px-4 py-2">
                  {editId === m.id ? (
                    <input
                      value={editValues[m.id]?.linux_prefix ?? m.linux_prefix}
                      onChange={e => setEditValues(ev => ({ ...ev, [m.id]: { ...ev[m.id], linux_prefix: e.target.value } }))}
                      className="bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-xs font-mono text-th-text w-full"
                    />
                  ) : (
                    <span className="font-mono text-xs text-th-text-secondary">{m.linux_prefix}</span>
                  )}
                </td>
                <td className="px-4 py-2">
                  {editId === m.id ? (
                    <input
                      type="checkbox"
                      checked={editValues[m.id]?.enabled ?? m.enabled}
                      onChange={e => setEditValues(ev => ({ ...ev, [m.id]: { ...ev[m.id], enabled: e.target.checked } }))}
                      className="rounded"
                    />
                  ) : (
                    <span className={m.enabled ? 'text-green-600 text-xs font-medium' : 'text-th-text-muted text-xs'}>
                      {m.enabled ? 'Yes' : 'No'}
                    </span>
                  )}
                </td>
                <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(m.updated_at)}</td>
                <td className="px-4 py-2 flex gap-2">
                  {editId === m.id ? (
                    <>
                      <button
                        onClick={() => handleSaveEdit(m)}
                        className="text-xs bg-green-100 text-green-800 px-2 py-0.5 rounded hover:bg-green-200"
                      >
                        Save
                      </button>
                      <button
                        onClick={() => setEditId(null)}
                        className="text-xs text-th-text-muted hover:underline"
                      >
                        Cancel
                      </button>
                    </>
                  ) : (
                    <>
                      <button
                        onClick={() => startEdit(m)}
                        className="text-xs bg-th-surface-muted text-th-text-secondary px-2 py-0.5 rounded hover:bg-th-border"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(m.id, m.name)}
                        className="text-xs text-red-600 hover:underline"
                      >
                        Delete
                      </button>
                    </>
                  )}
                </td>
              </tr>
            ))}
            {mappings.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-4 text-center text-th-text-subtle">No path mappings defined</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
