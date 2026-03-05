import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { Webhook, WebhookDelivery } from '../../types'

const PROVIDERS = ['discord', 'teams', 'slack', 'generic'] as const
const EVENT_OPTIONS = [
  'job.completed', 'job.failed', 'job.cancelled', 'agent.registered',
]

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

function DeliveryHistory({ webhookId }: { webhookId: string }) {
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.listWebhookDeliveries(webhookId)
      .then(setDeliveries)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [webhookId])

  if (loading) return <p className="text-xs text-th-text-muted px-4 py-2">Loading…</p>
  if (deliveries.length === 0) return <p className="text-xs text-th-text-subtle px-4 py-2">No deliveries recorded</p>

  return (
    <table className="w-full text-xs">
      <thead>
        <tr className="border-b border-th-border-subtle">
          <th className="px-4 py-1 text-left text-th-text-muted font-medium">Event</th>
          <th className="px-4 py-1 text-left text-th-text-muted font-medium">Status</th>
          <th className="px-4 py-1 text-left text-th-text-muted font-medium">Response</th>
          <th className="px-4 py-1 text-left text-th-text-muted font-medium">Attempt</th>
          <th className="px-4 py-1 text-left text-th-text-muted font-medium">Delivered</th>
        </tr>
      </thead>
      <tbody>
        {deliveries.map(d => (
          <tr key={d.id} className="border-b border-th-border-subtle last:border-0">
            <td className="px-4 py-1 text-th-text-secondary">{d.event}</td>
            <td className="px-4 py-1">
              <span className={d.success ? 'text-green-500' : 'text-red-500'}>
                {d.success ? '✓ OK' : '✗ Failed'}
              </span>
              {d.error_msg && <span className="ml-1 text-th-text-muted">({d.error_msg})</span>}
            </td>
            <td className="px-4 py-1 text-th-text-secondary">{d.response_code ?? '—'}</td>
            <td className="px-4 py-1 text-th-text-secondary">{d.attempt}</td>
            <td className="px-4 py-1 text-th-text-muted whitespace-nowrap">{fmtDate(d.delivered_at)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

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
  const [expandedDeliveries, setExpandedDeliveries] = useState<string | null>(null)

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

  const toggleDeliveries = (id: string) => {
    setExpandedDeliveries(prev => prev === id ? null : id)
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Webhooks</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          {showForm ? 'Cancel' : 'Add Webhook'}
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {showForm && (
        <form onSubmit={handleCreate} className="bg-th-surface rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-th-text-secondary">New Webhook</h2>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Name</label>
              <input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" required />
            </div>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Provider</label>
              <select value={form.provider} onChange={e => setForm(f => ({ ...f, provider: e.target.value as Webhook['provider'] }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
                {PROVIDERS.map(p => <option key={p} value={p}>{p}</option>)}
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">URL</label>
            <input type="url" value={form.url} onChange={e => setForm(f => ({ ...f, url: e.target.value }))}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" required />
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Secret (optional)</label>
            <input value={form.secret} onChange={e => setForm(f => ({ ...f, secret: e.target.value }))}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" />
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Events</label>
            <div className="flex flex-wrap gap-2">
              {EVENT_OPTIONS.map(ev => (
                <label key={ev} className="flex items-center gap-1 text-sm text-th-text-secondary cursor-pointer">
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

      <div className="space-y-2">
        {webhooks.length === 0 && (
          <p className="text-th-text-subtle text-sm text-center py-4">No webhooks configured</p>
        )}
        {webhooks.map(w => (
          <div key={w.id} className="bg-th-surface rounded-lg shadow overflow-hidden">
            <div className="flex items-center gap-3 px-4 py-3">
              <span className="font-medium text-th-text">{w.name}</span>
              <span className="text-xs text-th-text-muted">{w.provider}</span>
              <span className="text-xs text-th-text-muted max-w-xs truncate">{w.url}</span>
              <span className="text-xs text-th-text-muted">{w.events.join(', ') || '—'}</span>
              <span className={`text-xs font-medium ml-auto ${w.enabled ? 'text-green-600' : 'text-th-text-subtle'}`}>
                {w.enabled ? 'Enabled' : 'Disabled'}
              </span>
              <button onClick={() => toggleDeliveries(w.id)}
                className="text-xs text-th-text-muted hover:text-th-text">
                {expandedDeliveries === w.id ? '▲ Hide History' : '▼ History'}
              </button>
              <button onClick={() => handleTest(w.id)} disabled={testing === w.id}
                className="text-xs px-2 py-0.5 rounded disabled:opacity-50"
                style={{ backgroundColor: 'var(--th-badge-running-bg)', color: 'var(--th-badge-running-text)' }}
              >
                {testing === w.id ? 'Testing…' : 'Test'}
              </button>
              <button onClick={() => handleDelete(w.id)}
                className="text-xs text-red-600 hover:underline">Delete</button>
            </div>
            {expandedDeliveries === w.id && (
              <div className="border-t border-th-border">
                <DeliveryHistory webhookId={w.id} />
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
