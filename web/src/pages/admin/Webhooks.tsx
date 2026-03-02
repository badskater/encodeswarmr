import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { Webhook } from '../../types'

const PROVIDERS = ['discord', 'teams', 'slack', 'generic'] as const
const EVENT_OPTIONS = [
  'job.completed', 'job.failed', 'job.cancelled', 'agent.registered',
]

export default function Webhooks() {
  const [webhooks, setWebhooks] = useState<Webhook[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [form, setForm] = useState({
    name: '', provider: 'discord' as Webhook['provider'], url: '', secret: '',
    events: [] as string[],
  })
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const w = await api.listWebhooks()
      setWebhooks(w)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const toggleEvent = (ev: string) => {
    setForm(f => ({
      ...f,
      events: f.events.includes(ev) ? f.events.filter(e => e !== ev) : [...f.events, ev],
    }))
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    try {
      const body: Partial<Webhook> & { secret?: string } = {
        name: form.name, provider: form.provider, url: form.url, events: form.events,
      }
      if (form.secret) (body as Record<string, unknown>).secret = form.secret
      await api.createWebhook(body)
      setShowForm(false)
      setForm({ name: '', provider: 'discord', url: '', secret: '', events: [] })
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create webhook')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this webhook?')) return
    try {
      await api.deleteWebhook(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  const handleTest = async (id: string) => {
    setTesting(id)
    try {
      await api.testWebhook(id)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Test failed')
    } finally {
      setTesting(null)
    }
  }

  if (loading) return <p className="text-gray-500">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Webhooks</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          {showForm ? 'Cancel' : 'Add Webhook'}
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {showForm && (
        <form onSubmit={handleCreate} className="bg-white rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-gray-700">New Webhook</h2>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-gray-500 mb-1">Name</label>
              <input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm" required />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Provider</label>
              <select value={form.provider} onChange={e => setForm(f => ({ ...f, provider: e.target.value as Webhook['provider'] }))}
                className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm">
                {PROVIDERS.map(p => <option key={p} value={p}>{p}</option>)}
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">URL</label>
            <input type="url" value={form.url} onChange={e => setForm(f => ({ ...f, url: e.target.value }))}
              className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm" required />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Secret (optional)</label>
            <input value={form.secret} onChange={e => setForm(f => ({ ...f, secret: e.target.value }))}
              className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm" />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Events</label>
            <div className="flex flex-wrap gap-2">
              {EVENT_OPTIONS.map(ev => (
                <label key={ev} className="flex items-center gap-1 text-sm cursor-pointer">
                  <input type="checkbox" checked={form.events.includes(ev)} onChange={() => toggleEvent(ev)} />
                  {ev}
                </label>
              ))}
            </div>
          </div>
          <button type="submit" disabled={saving}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
            {saving ? 'Saving…' : 'Create Webhook'}
          </button>
        </form>
      )}

      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200 text-sm">
          <thead className="bg-gray-50">
            <tr>
              {['Name', 'Provider', 'URL', 'Events', 'Enabled', ''].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {webhooks.map(w => (
              <tr key={w.id} className="hover:bg-gray-50">
                <td className="px-4 py-2 font-medium text-gray-900">{w.name}</td>
                <td className="px-4 py-2 text-gray-700">{w.provider}</td>
                <td className="px-4 py-2 text-gray-500 max-w-xs truncate">{w.url}</td>
                <td className="px-4 py-2 text-gray-500 text-xs">{w.events.join(', ') || '—'}</td>
                <td className="px-4 py-2">
                  <span className={`text-xs font-medium ${w.enabled ? 'text-green-600' : 'text-gray-400'}`}>
                    {w.enabled ? 'Enabled' : 'Disabled'}
                  </span>
                </td>
                <td className="px-4 py-2 flex gap-2">
                  <button onClick={() => handleTest(w.id)} disabled={testing === w.id}
                    className="text-xs bg-blue-100 text-blue-800 px-2 py-0.5 rounded hover:bg-blue-200 disabled:opacity-50">
                    {testing === w.id ? 'Testing…' : 'Test'}
                  </button>
                  <button onClick={() => handleDelete(w.id)}
                    className="text-xs text-red-600 hover:underline">Delete</button>
                </td>
              </tr>
            ))}
            {webhooks.length === 0 && (
              <tr><td colSpan={6} className="px-4 py-4 text-center text-gray-400">No webhooks configured</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
