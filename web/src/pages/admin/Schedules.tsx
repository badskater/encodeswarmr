import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { Schedule } from '../../types'

const EMPTY_TEMPLATE = JSON.stringify(
  { source_id: '', job_type: 'encode', priority: 0, target_tags: [] },
  null,
  2,
)

function fmtDate(s: string | null | undefined) {
  if (!s) return '—'
  return new Date(s).toLocaleString()
}

interface FormState {
  name: string
  cron_expr: string
  job_template: string
  enabled: boolean
}

const EMPTY_FORM: FormState = {
  name: '',
  cron_expr: '0 2 * * *',
  job_template: EMPTY_TEMPLATE,
  enabled: true,
}

export default function Schedules() {
  const [schedules, setSchedules] = useState<Schedule[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [editTarget, setEditTarget] = useState<Schedule | null>(null)
  const [form, setForm] = useState<FormState>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [templateError, setTemplateError] = useState('')

  const load = useCallback(async () => {
    try {
      const data = await api.listSchedules()
      setSchedules(data)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load schedules')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const openCreate = () => {
    setEditTarget(null)
    setForm(EMPTY_FORM)
    setTemplateError('')
    setShowForm(true)
  }

  const openEdit = (sc: Schedule) => {
    setEditTarget(sc)
    setForm({
      name: sc.name,
      cron_expr: sc.cron_expr,
      job_template: JSON.stringify(sc.job_template, null, 2),
      enabled: sc.enabled,
    })
    setTemplateError('')
    setShowForm(true)
  }

  const closeForm = () => {
    setShowForm(false)
    setEditTarget(null)
    setForm(EMPTY_FORM)
    setTemplateError('')
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setTemplateError('')

    // Validate that job_template is valid JSON.
    let parsed: unknown
    try {
      parsed = JSON.parse(form.job_template)
    } catch {
      setTemplateError('Job template must be valid JSON')
      return
    }

    setSaving(true)
    try {
      const body = {
        name: form.name,
        cron_expr: form.cron_expr,
        job_template: parsed,
        enabled: form.enabled,
      }
      if (editTarget) {
        await api.updateSchedule(editTarget.id, body)
      } else {
        await api.createSchedule(body)
      }
      closeForm()
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save schedule')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this schedule?')) return
    try {
      await api.deleteSchedule(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete schedule')
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Schedules</h1>
        <button
          onClick={openCreate}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          New Schedule
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {showForm && (
        <form onSubmit={handleSubmit} className="bg-th-surface rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-th-text-secondary">
            {editTarget ? 'Edit Schedule' : 'New Schedule'}
          </h2>

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
            <div>
              <label className="block text-xs text-th-text-muted mb-1">
                Cron Expression
                <span className="ml-1 text-th-text-subtle font-normal">(e.g. 0 2 * * *)</span>
              </label>
              <input
                value={form.cron_expr}
                onChange={e => setForm(f => ({ ...f, cron_expr: e.target.value }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text font-mono"
                required
              />
            </div>
          </div>

          <div>
            <label className="block text-xs text-th-text-muted mb-1">
              Job Template <span className="font-normal text-th-text-subtle">(JSON — CreateJobParams)</span>
            </label>
            <textarea
              value={form.job_template}
              onChange={e => setForm(f => ({ ...f, job_template: e.target.value }))}
              rows={8}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text font-mono"
              required
              spellCheck={false}
            />
            {templateError && <p className="text-red-500 text-xs mt-1">{templateError}</p>}
          </div>

          <label className="flex items-center gap-2 text-sm text-th-text-secondary cursor-pointer select-none">
            <input
              type="checkbox"
              checked={form.enabled}
              onChange={e => setForm(f => ({ ...f, enabled: e.target.checked }))}
            />
            Enabled
          </label>

          <div className="flex gap-2">
            <button
              type="submit"
              disabled={saving}
              className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
            >
              {saving ? 'Saving…' : editTarget ? 'Update' : 'Create'}
            </button>
            <button
              type="button"
              onClick={closeForm}
              className="px-3 py-1.5 rounded text-sm font-medium text-th-text-secondary hover:bg-th-nav-hover hover:text-white"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {schedules.length === 0 ? (
        <p className="text-th-text-subtle text-sm">No schedules configured.</p>
      ) : (
        <div className="bg-th-surface rounded-lg shadow overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-th-border-subtle">
                <th className="px-4 py-2 text-left text-th-text-muted font-medium">Name</th>
                <th className="px-4 py-2 text-left text-th-text-muted font-medium">Cron Expression</th>
                <th className="px-4 py-2 text-left text-th-text-muted font-medium">Next Run</th>
                <th className="px-4 py-2 text-left text-th-text-muted font-medium">Last Run</th>
                <th className="px-4 py-2 text-left text-th-text-muted font-medium">Enabled</th>
                <th className="px-4 py-2 text-right text-th-text-muted font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {schedules.map(sc => (
                <tr key={sc.id} className="border-b border-th-border-subtle last:border-0">
                  <td className="px-4 py-2 text-th-text font-medium">{sc.name}</td>
                  <td className="px-4 py-2 text-th-text-secondary font-mono text-xs">{sc.cron_expr}</td>
                  <td className="px-4 py-2 text-th-text-secondary whitespace-nowrap">{fmtDate(sc.next_run_at)}</td>
                  <td className="px-4 py-2 text-th-text-secondary whitespace-nowrap">{fmtDate(sc.last_run_at)}</td>
                  <td className="px-4 py-2">
                    <span className={`text-xs font-medium ${sc.enabled ? 'text-green-500' : 'text-th-text-muted'}`}>
                      {sc.enabled ? 'Yes' : 'No'}
                    </span>
                  </td>
                  <td className="px-4 py-2 text-right">
                    <div className="flex justify-end gap-2">
                      <button
                        onClick={() => openEdit(sc)}
                        className="text-xs text-blue-500 hover:text-blue-400"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(sc.id)}
                        className="text-xs text-red-500 hover:text-red-400"
                      >
                        Delete
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
