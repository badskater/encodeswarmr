import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { Template } from '../../types'

interface RuleCondition {
  field: string
  operator: string
  value: string
}

interface RuleAction {
  suggest_template_id?: string
  suggest_audio_codec?: string
  suggest_priority?: number
  suggest_tags?: string[]
}

interface EncodingRule {
  id: string
  name: string
  priority: number
  conditions: RuleCondition[]
  actions: RuleAction
  enabled: boolean
  created_at: string
  updated_at: string
}

const FIELD_OPTIONS = [
  { value: 'resolution', label: 'Resolution' },
  { value: 'hdr_type', label: 'HDR Type' },
  { value: 'codec', label: 'Codec' },
  { value: 'file_size_gb', label: 'File Size (GB)' },
  { value: 'duration_min', label: 'Duration (min)' },
]

const OPERATOR_OPTIONS = [
  { value: 'eq', label: '=' },
  { value: 'neq', label: '≠' },
  { value: 'gt', label: '>' },
  { value: 'lt', label: '<' },
  { value: 'gte', label: '>=' },
  { value: 'lte', label: '<=' },
  { value: 'contains', label: 'contains' },
  { value: 'in', label: 'in (comma-separated)' },
]

const emptyRule = (): Omit<EncodingRule, 'id' | 'created_at' | 'updated_at'> => ({
  name: '',
  priority: 100,
  conditions: [{ field: 'resolution', operator: 'eq', value: '' }],
  actions: {},
  enabled: true,
})

export default function EncodingRules() {
  const [rules, setRules] = useState<EncodingRule[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editing, setEditing] = useState<Partial<EncodingRule> | null>(null)
  const [saving, setSaving] = useState(false)
  const [deleting, setDeleting] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const [r, t] = await Promise.all([api.listEncodingRules(), api.listTemplates()])
      setRules(r)
      setTemplates(t)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editing) return
    setSaving(true)
    try {
      if (editing.id) {
        await api.updateEncodingRule(editing.id, editing)
      } else {
        await api.createEncodingRule(editing)
      }
      setEditing(null)
      await load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save rule')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this rule?')) return
    setDeleting(id)
    try {
      await api.deleteEncodingRule(id)
      await load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete rule')
    } finally {
      setDeleting(null)
    }
  }

  const addCondition = () => {
    if (!editing) return
    setEditing({
      ...editing,
      conditions: [...(editing.conditions ?? []), { field: 'resolution', operator: 'eq', value: '' }],
    })
  }

  const removeCondition = (idx: number) => {
    if (!editing) return
    const conds = [...(editing.conditions ?? [])]
    conds.splice(idx, 1)
    setEditing({ ...editing, conditions: conds })
  }

  const updateCondition = (idx: number, key: keyof RuleCondition, val: string) => {
    if (!editing) return
    const conds = [...(editing.conditions ?? [])] as RuleCondition[]
    conds[idx] = { ...conds[idx], [key]: val }
    setEditing({ ...editing, conditions: conds })
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Encoding Rules</h1>
        <button
          onClick={() => setEditing(emptyRule())}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          New Rule
        </button>
      </div>

      <div className="bg-th-surface rounded-lg shadow p-4 text-sm text-th-text-muted">
        Rules are evaluated when creating a job to <strong className="text-th-text">suggest</strong> a
        template, audio codec, priority, and tags based on source properties. Suggestions are never
        auto-applied — you always confirm before use.
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {editing && (
        <form onSubmit={handleSave} className="bg-th-surface rounded-lg shadow p-4 space-y-4">
          <h2 className="text-sm font-semibold text-th-text-secondary">
            {editing.id ? 'Edit Rule' : 'New Rule'}
          </h2>

          {/* Name + Priority + Enabled */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div className="sm:col-span-2">
              <label className="block text-xs text-th-text-muted mb-1">Name *</label>
              <input
                value={editing.name ?? ''}
                onChange={e => setEditing({ ...editing, name: e.target.value })}
                required
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
              />
            </div>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Priority</label>
              <input
                type="number"
                value={editing.priority ?? 100}
                onChange={e => setEditing({ ...editing, priority: parseInt(e.target.value) || 100 })}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
              />
              <p className="text-xs text-th-text-muted mt-0.5">Lower = higher priority</p>
            </div>
          </div>

          <label className="flex items-center gap-2 text-sm cursor-pointer">
            <input
              type="checkbox"
              checked={editing.enabled ?? true}
              onChange={e => setEditing({ ...editing, enabled: e.target.checked })}
              className="accent-blue-600"
            />
            <span className="text-th-text">Enabled</span>
          </label>

          {/* Conditions */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="text-xs font-medium text-th-text-muted uppercase tracking-wide">Conditions (all must match)</label>
              <button type="button" onClick={addCondition} className="text-xs text-blue-600 hover:text-blue-700">+ Add</button>
            </div>
            {(editing.conditions ?? []).map((c, idx) => (
              <div key={idx} className="flex items-center gap-2">
                <select
                  value={c.field}
                  onChange={e => updateCondition(idx, 'field', e.target.value)}
                  className="bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                >
                  {FIELD_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
                <select
                  value={c.operator}
                  onChange={e => updateCondition(idx, 'operator', e.target.value)}
                  className="bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                >
                  {OPERATOR_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
                <input
                  value={c.value}
                  onChange={e => updateCondition(idx, 'value', e.target.value)}
                  placeholder="value"
                  className="flex-1 bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                />
                <button type="button" onClick={() => removeCondition(idx)} className="text-red-500 hover:text-red-700 text-sm px-1">✕</button>
              </div>
            ))}
          </div>

          {/* Actions */}
          <div className="space-y-2">
            <label className="text-xs font-medium text-th-text-muted uppercase tracking-wide">Suggested Actions</label>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Suggest Template</label>
                <select
                  value={editing.actions?.suggest_template_id ?? ''}
                  onChange={e => setEditing({ ...editing, actions: { ...editing.actions, suggest_template_id: e.target.value || undefined } })}
                  className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                >
                  <option value="">— none —</option>
                  {templates.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
                </select>
              </div>
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Suggest Audio Codec</label>
                <select
                  value={editing.actions?.suggest_audio_codec ?? ''}
                  onChange={e => setEditing({ ...editing, actions: { ...editing.actions, suggest_audio_codec: e.target.value || undefined } })}
                  className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                >
                  <option value="">— none —</option>
                  {['flac', 'libopus', 'aac', 'libfdk_aac', 'ac3', 'eac3', 'truehd'].map(c => (
                    <option key={c} value={c}>{c}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Suggest Priority</label>
                <input
                  type="number"
                  value={editing.actions?.suggest_priority ?? ''}
                  onChange={e => setEditing({ ...editing, actions: { ...editing.actions, suggest_priority: parseInt(e.target.value) || undefined } })}
                  placeholder="e.g. 10"
                  className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                />
              </div>
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Suggest Tags (comma-separated)</label>
                <input
                  value={(editing.actions?.suggest_tags ?? []).join(', ')}
                  onChange={e => setEditing({ ...editing, actions: { ...editing.actions, suggest_tags: e.target.value.split(',').map(s => s.trim()).filter(Boolean) } })}
                  placeholder="e.g. hdr, 4k"
                  className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                />
              </div>
            </div>
          </div>

          <div className="flex gap-2">
            <button type="submit" disabled={saving}
              className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
              {saving ? 'Saving…' : 'Save'}
            </button>
            <button type="button" onClick={() => setEditing(null)}
              className="border border-th-border text-th-text px-4 py-2 rounded text-sm hover:bg-th-surface-muted">
              Cancel
            </button>
          </div>
        </form>
      )}

      {rules.length === 0 && !editing ? (
        <p className="text-center text-th-text-subtle text-sm py-8">No rules defined yet.</p>
      ) : (
        <div className="bg-th-surface rounded-lg shadow overflow-hidden">
          <table className="min-w-full divide-y divide-th-border text-sm">
            <thead className="bg-th-surface-muted">
              <tr>
                {['Priority', 'Name', 'Conditions', 'Suggestion', 'Enabled', ''].map(h => (
                  <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-th-border-subtle">
              {rules.map(r => (
                <tr key={r.id} className="hover:bg-th-surface-muted">
                  <td className="px-4 py-2 text-th-text font-mono">{r.priority}</td>
                  <td className="px-4 py-2 font-medium text-th-text">{r.name}</td>
                  <td className="px-4 py-2 text-th-text-muted text-xs max-w-xs">
                    {r.conditions.map((c, i) => (
                      <span key={i} className="inline-block mr-1 mb-0.5 bg-th-surface-muted rounded px-1.5 py-0.5">
                        {c.field} {c.operator} {c.value}
                      </span>
                    ))}
                  </td>
                  <td className="px-4 py-2 text-th-text-muted text-xs">
                    {r.actions.suggest_template_id && (
                      <div>Template: {templates.find(t => t.id === r.actions.suggest_template_id)?.name ?? r.actions.suggest_template_id}</div>
                    )}
                    {r.actions.suggest_audio_codec && <div>Audio: {r.actions.suggest_audio_codec}</div>}
                    {r.actions.suggest_priority != null && r.actions.suggest_priority !== 0 && (
                      <div>Priority: {r.actions.suggest_priority}</div>
                    )}
                  </td>
                  <td className="px-4 py-2">
                    <span className={`text-xs font-medium ${r.enabled ? 'text-green-600' : 'text-gray-400'}`}>
                      {r.enabled ? 'Yes' : 'No'}
                    </span>
                  </td>
                  <td className="px-4 py-2">
                    <div className="flex gap-2">
                      <button onClick={() => setEditing(r)}
                        className="text-xs text-blue-600 hover:text-blue-700">Edit</button>
                      <button onClick={() => handleDelete(r.id)} disabled={deleting === r.id}
                        className="text-xs text-red-600 hover:text-red-700 disabled:opacity-50">
                        {deleting === r.id ? '…' : 'Delete'}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
