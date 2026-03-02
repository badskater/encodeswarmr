import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { Variable } from '../../types'

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

export default function Variables() {
  const [variables, setVariables] = useState<Variable[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({ name: '', value: '', description: '' })
  const [saving, setSaving] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [editValues, setEditValues] = useState<Record<string, { value: string; description: string }>>({})

  const load = useCallback(async () => {
    try {
      const v = await api.listVariables()
      setVariables(v)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleUpsert = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    try {
      await api.upsertVariable(form.name, form.value, form.description || undefined)
      setShowForm(false)
      setForm({ name: '', value: '', description: '' })
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save variable')
    } finally {
      setSaving(false)
    }
  }

  const handleSaveEdit = async (v: Variable) => {
    const ed = editValues[v.id]
    if (!ed) return
    try {
      await api.upsertVariable(v.name, ed.value, ed.description || undefined)
      setEditId(null)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to update')
    }
  }

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete variable "${name}"?`)) return
    try {
      await api.deleteVariable(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  const startEdit = (v: Variable) => {
    setEditId(v.id)
    setEditValues(ev => ({ ...ev, [v.id]: { value: v.value, description: v.description ?? '' } }))
  }

  if (loading) return <p className="text-gray-500">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Variables</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          {showForm ? 'Cancel' : 'Add Variable'}
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {showForm && (
        <form onSubmit={handleUpsert} className="bg-white rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-gray-700">New / Update Variable</h2>
          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="block text-xs text-gray-500 mb-1">Name</label>
              <input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm font-mono" required />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Value</label>
              <input value={form.value} onChange={e => setForm(f => ({ ...f, value: e.target.value }))}
                className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm font-mono" required />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Description</label>
              <input value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
                className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm" />
            </div>
          </div>
          <button type="submit" disabled={saving}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
            {saving ? 'Saving…' : 'Save Variable'}
          </button>
        </form>
      )}

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200 text-sm">
          <thead className="bg-gray-50">
            <tr>
              {['Name', 'Value', 'Description', 'Updated', ''].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {variables.map(v => (
              <tr key={v.id} className="hover:bg-gray-50">
                <td className="px-4 py-2 font-mono text-xs font-medium text-gray-900">{v.name}</td>
                <td className="px-4 py-2">
                  {editId === v.id ? (
                    <input
                      value={editValues[v.id]?.value ?? v.value}
                      onChange={e => setEditValues(ev => ({ ...ev, [v.id]: { ...ev[v.id], value: e.target.value } }))}
                      className="border border-gray-300 rounded px-2 py-1 text-xs font-mono w-full"
                    />
                  ) : (
                    <span className="font-mono text-xs text-gray-700">{v.value}</span>
                  )}
                </td>
                <td className="px-4 py-2">
                  {editId === v.id ? (
                    <input
                      value={editValues[v.id]?.description ?? ''}
                      onChange={e => setEditValues(ev => ({ ...ev, [v.id]: { ...ev[v.id], description: e.target.value } }))}
                      className="border border-gray-300 rounded px-2 py-1 text-xs w-full"
                    />
                  ) : (
                    <span className="text-gray-500">{v.description ?? '—'}</span>
                  )}
                </td>
                <td className="px-4 py-2 text-gray-500 whitespace-nowrap">{fmtDate(v.updated_at)}</td>
                <td className="px-4 py-2 flex gap-2">
                  {editId === v.id ? (
                    <>
                      <button onClick={() => handleSaveEdit(v)}
                        className="text-xs bg-green-100 text-green-800 px-2 py-0.5 rounded hover:bg-green-200">Save</button>
                      <button onClick={() => setEditId(null)}
                        className="text-xs text-gray-500 hover:underline">Cancel</button>
                    </>
                  ) : (
                    <>
                      <button onClick={() => startEdit(v)}
                        className="text-xs bg-gray-100 text-gray-700 px-2 py-0.5 rounded hover:bg-gray-200">Edit</button>
                      <button onClick={() => handleDelete(v.id, v.name)}
                        className="text-xs text-red-600 hover:underline">Delete</button>
                    </>
                  )}
                </td>
              </tr>
            ))}
            {variables.length === 0 && (
              <tr><td colSpan={5} className="px-4 py-4 text-center text-gray-400">No variables defined</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
