import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { Template } from '../../types'

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

export default function Templates() {
  const [templates, setTemplates] = useState<Template[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ name: '', type: 'bat' as Template['type'], description: '', content: '' })
  const [saving, setSaving] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [editValues, setEditValues] = useState<{ name: string; description: string; content: string }>({ name: '', description: '', content: '' })
  const [editSaving, setEditSaving] = useState(false)

  const load = useCallback(async () => {
    try {
      const t = await api.listTemplates()
      setTemplates(t)
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
      await api.createTemplate(form)
      setShowForm(false)
      setForm({ name: '', type: 'bat', description: '', content: '' })
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create')
    } finally {
      setSaving(false)
    }
  }

  const startEdit = (t: Template) => {
    setEditId(t.id)
    setEditValues({ name: t.name, description: t.description ?? '', content: t.content })
  }

  const handleSaveEdit = async (id: string) => {
    setEditSaving(true)
    try {
      await api.updateTemplate(id, editValues)
      setEditId(null)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save')
    } finally {
      setEditSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this template?')) return
    try {
      await api.deleteTemplate(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Templates</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          {showForm ? 'Cancel' : 'Add Template'}
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {showForm && (
        <form onSubmit={handleCreate} className="bg-th-surface rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-th-text-secondary">New Template</h2>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Name</label>
              <input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" required />
            </div>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Type</label>
              <select value={form.type} onChange={e => setForm(f => ({ ...f, type: e.target.value as Template['type'] }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
                <option value="bat">.bat</option>
                <option value="avs">.avs</option>
                <option value="vpy">.vpy</option>
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Description</label>
            <input value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" />
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Content</label>
            <textarea value={form.content} onChange={e => setForm(f => ({ ...f, content: e.target.value }))}
              rows={8}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm font-mono text-th-text" required />
          </div>
          <button type="submit" disabled={saving}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
            {saving ? 'Saving…' : 'Create Template'}
          </button>
        </form>
      )}

      <div className="space-y-3">
        {templates.length === 0 && !loading && (
          <p className="text-th-text-subtle text-sm text-center py-4">No templates</p>
        )}
        {templates.map(t => (
          <div key={t.id} className="bg-th-surface rounded-lg shadow p-4">
            {editId === t.id ? (
              <div className="space-y-3">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs text-th-text-muted mb-1">Name</label>
                    <input
                      value={editValues.name}
                      onChange={e => setEditValues(v => ({ ...v, name: e.target.value }))}
                      className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-th-text-muted mb-1">Description</label>
                    <input
                      value={editValues.description}
                      onChange={e => setEditValues(v => ({ ...v, description: e.target.value }))}
                      className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-xs text-th-text-muted mb-1">Content</label>
                  <textarea
                    value={editValues.content}
                    onChange={e => setEditValues(v => ({ ...v, content: e.target.value }))}
                    rows={12}
                    className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm font-mono text-th-text"
                  />
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => handleSaveEdit(t.id)}
                    disabled={editSaving}
                    className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
                  >
                    {editSaving ? 'Saving…' : 'Save'}
                  </button>
                  <button
                    onClick={() => setEditId(null)}
                    className="px-3 py-1.5 rounded text-sm text-th-text-muted border border-th-border hover:bg-th-surface-muted"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            ) : (
              <div>
                <div className="flex items-start justify-between mb-2">
                  <div>
                    <span className="font-medium text-th-text">{t.name}</span>
                    <span className="ml-2 font-mono text-xs bg-th-surface-muted px-1.5 py-0.5 rounded text-th-text-muted">.{t.type}</span>
                    {t.description && <span className="ml-2 text-sm text-th-text-muted">{t.description}</span>}
                  </div>
                  <div className="flex items-center gap-3 shrink-0">
                    <span className="text-xs text-th-text-subtle">{fmtDate(t.created_at)}</span>
                    <button onClick={() => startEdit(t)} className="text-xs text-blue-600 hover:underline">Edit</button>
                    <button onClick={() => handleDelete(t.id)} className="text-xs text-red-600 hover:underline">Delete</button>
                  </div>
                </div>
                <pre className="bg-th-log-bg rounded p-3 text-xs font-mono text-th-log-text overflow-auto max-h-40">{t.content}</pre>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
