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

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this template?')) return
    try {
      await api.deleteTemplate(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  if (loading) return <p className="text-gray-500">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Templates</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          {showForm ? 'Cancel' : 'Add Template'}
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {showForm && (
        <form onSubmit={handleCreate} className="bg-white rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-gray-700">New Template</h2>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-gray-500 mb-1">Name</label>
              <input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm" required />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Type</label>
              <select value={form.type} onChange={e => setForm(f => ({ ...f, type: e.target.value as Template['type'] }))}
                className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm">
                <option value="bat">.bat</option>
                <option value="avs">.avs</option>
                <option value="vpy">.vpy</option>
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Description</label>
            <input value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
              className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm" />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Content</label>
            <textarea value={form.content} onChange={e => setForm(f => ({ ...f, content: e.target.value }))}
              rows={8}
              className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm font-mono" required />
          </div>
          <button type="submit" disabled={saving}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
            {saving ? 'Saving…' : 'Create Template'}
          </button>
        </form>
      )}

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200 text-sm">
          <thead className="bg-gray-50">
            <tr>
              {['Name', 'Type', 'Description', 'Created', ''].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {templates.map(t => (
              <tr key={t.id} className="hover:bg-gray-50">
                <td className="px-4 py-2 font-medium text-gray-900">{t.name}</td>
                <td className="px-4 py-2"><span className="font-mono text-xs bg-gray-100 px-1.5 py-0.5 rounded">.{t.type}</span></td>
                <td className="px-4 py-2 text-gray-500">{t.description ?? '—'}</td>
                <td className="px-4 py-2 text-gray-500 whitespace-nowrap">{fmtDate(t.created_at)}</td>
                <td className="px-4 py-2">
                  <button onClick={() => handleDelete(t.id)}
                    className="text-xs text-red-600 hover:underline">Delete</button>
                </td>
              </tr>
            ))}
            {templates.length === 0 && (
              <tr><td colSpan={5} className="px-4 py-4 text-center text-gray-400">No templates</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
