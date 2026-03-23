import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { EncodingProfile, Template } from '../../types'

const AUDIO_CODECS = ['', 'flac', 'libopus', 'libfdk_aac', 'aac', 'ac3', 'eac3', 'dca', 'truehd', 'pcm_s16le', 'pcm_s24le', 'libmp3lame']

const EMPTY_FORM: Partial<EncodingProfile> = {
  name: '',
  description: '',
  run_template_id: '',
  frameserver_template_id: '',
  audio_codec: '',
  audio_bitrate: '',
  output_extension: 'mkv',
  output_path_pattern: '',
  target_tags: [],
  priority: 5,
  enabled: true,
}

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

export default function EncodingProfiles() {
  const [profiles, setProfiles] = useState<EncodingProfile[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [form, setForm] = useState<Partial<EncodingProfile>>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [tagsInput, setTagsInput] = useState('')

  const load = useCallback(async () => {
    try {
      const [p, t] = await Promise.all([
        api.listEncodingProfiles(),
        api.listTemplates(),
      ])
      setProfiles(p ?? [])
      setTemplates(t ?? [])
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const openCreate = () => {
    setEditingId(null)
    setForm(EMPTY_FORM)
    setTagsInput('')
    setShowForm(true)
  }

  const openEdit = (p: EncodingProfile) => {
    setEditingId(p.id)
    setForm({ ...p })
    setTagsInput(p.target_tags.join(', '))
    setShowForm(true)
  }

  const handleSave = async () => {
    setSaving(true)
    setError('')
    try {
      const payload: Partial<EncodingProfile> = {
        ...form,
        target_tags: tagsInput.split(',').map(t => t.trim()).filter(Boolean),
      }
      if (editingId) {
        await api.updateEncodingProfile(editingId, payload)
      } else {
        await api.createEncodingProfile(payload)
      }
      setShowForm(false)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this encoding profile?')) return
    try {
      await api.deleteEncodingProfile(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  const field = (key: keyof EncodingProfile) => ({
    value: (form[key] as string) ?? '',
    onChange: (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) =>
      setForm(prev => ({ ...prev, [key]: e.target.value })),
  })

  const runTemplates = templates.filter(t => t.type === 'run')
  const fsTemplates = templates.filter(t => t.type === 'frameserver')

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-th-text">Encoding Profiles</h1>
          <p className="text-sm text-th-text-muted mt-0.5">
            Bundles a run template, audio codec, output pattern, and tags into a single named preset.
          </p>
        </div>
        <button
          onClick={openCreate}
          className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
        >
          New Profile
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {/* Profile list */}
      <div className="bg-th-surface rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-th-border text-sm">
          <thead className="bg-th-surface-muted">
            <tr>
              {['Name', 'Run Template', 'Audio Codec', 'Extension', 'Tags', 'Priority', 'Enabled', 'Updated', ''].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-th-border-subtle">
            {profiles.map(p => {
              const runTpl = templates.find(t => t.id === p.run_template_id)
              return (
                <tr key={p.id} className="hover:bg-th-surface-muted">
                  <td className="px-4 py-2 font-medium text-th-text">{p.name}</td>
                  <td className="px-4 py-2 text-th-text-secondary">{runTpl?.name ?? p.run_template_id.slice(0, 8) + '…'}</td>
                  <td className="px-4 py-2 text-th-text-secondary">{p.audio_codec || '—'}</td>
                  <td className="px-4 py-2 text-th-text-secondary">{p.output_extension}</td>
                  <td className="px-4 py-2 text-th-text-muted text-xs">
                    {p.target_tags.length > 0 ? p.target_tags.join(', ') : '—'}
                  </td>
                  <td className="px-4 py-2 text-th-text-secondary">{p.priority}</td>
                  <td className="px-4 py-2">
                    <span className={`text-xs font-medium ${p.enabled ? 'text-green-600' : 'text-th-text-muted'}`}>
                      {p.enabled ? 'Yes' : 'No'}
                    </span>
                  </td>
                  <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(p.updated_at)}</td>
                  <td className="px-4 py-2 flex gap-2">
                    <button
                      onClick={() => openEdit(p)}
                      className="text-xs px-2 py-1 rounded border border-th-border text-th-text hover:bg-th-surface-muted"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => handleDelete(p.id)}
                      className="text-xs px-2 py-1 rounded border border-red-200 text-red-600 hover:bg-red-50 dark:hover:bg-red-950"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              )
            })}
            {profiles.length === 0 && (
              <tr>
                <td colSpan={9} className="px-4 py-6 text-center text-th-text-subtle">
                  No encoding profiles yet. Create one to bundle templates and settings into a reusable preset.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Create / edit modal */}
      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50">
          <div className="bg-th-surface rounded-xl shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
            <div className="px-6 py-4 border-b border-th-border flex items-center justify-between">
              <h2 className="text-base font-semibold text-th-text">
                {editingId ? 'Edit Profile' : 'New Encoding Profile'}
              </h2>
              <button onClick={() => setShowForm(false)} className="text-th-text-muted hover:text-th-text text-xl leading-none">×</button>
            </div>
            <div className="px-6 py-5 space-y-4">
              {error && <p className="text-red-600 text-sm">{error}</p>}

              <FormRow label="Name *">
                <input className={inputCls} {...field('name')} placeholder="4K HEVC Encode" />
              </FormRow>
              <FormRow label="Description">
                <textarea
                  className={inputCls}
                  value={form.description ?? ''}
                  onChange={e => setForm(prev => ({ ...prev, description: e.target.value }))}
                  rows={2}
                  placeholder="Optional description"
                />
              </FormRow>
              <FormRow label="Run Template *">
                <select className={inputCls} {...field('run_template_id')}>
                  <option value="">— select —</option>
                  {runTemplates.map(t => (
                    <option key={t.id} value={t.id}>{t.name}</option>
                  ))}
                  {runTemplates.length === 0 && (
                    <option disabled>No run templates found</option>
                  )}
                </select>
              </FormRow>
              <FormRow label="Frameserver Template">
                <select className={inputCls} {...field('frameserver_template_id')}>
                  <option value="">— none —</option>
                  {fsTemplates.map(t => (
                    <option key={t.id} value={t.id}>{t.name}</option>
                  ))}
                </select>
              </FormRow>
              <div className="grid grid-cols-2 gap-4">
                <FormRow label="Audio Codec">
                  <select className={inputCls} {...field('audio_codec')}>
                    {AUDIO_CODECS.map(c => (
                      <option key={c} value={c}>{c || '— none —'}</option>
                    ))}
                  </select>
                </FormRow>
                <FormRow label="Audio Bitrate">
                  <input className={inputCls} {...field('audio_bitrate')} placeholder="320k" />
                </FormRow>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <FormRow label="Output Extension">
                  <input className={inputCls} {...field('output_extension')} placeholder="mkv" />
                </FormRow>
                <FormRow label="Priority">
                  <input
                    type="number"
                    className={inputCls}
                    value={form.priority ?? 5}
                    min={1} max={10}
                    onChange={e => setForm(prev => ({ ...prev, priority: parseInt(e.target.value, 10) || 5 }))}
                  />
                </FormRow>
              </div>
              <FormRow label="Output Path Pattern" hint="Variables: {source_name}, {date}, {codec}">
                <input className={inputCls} {...field('output_path_pattern')} placeholder="\\NAS\encoded\{date}\{source_name}" />
              </FormRow>
              <FormRow label="Target Tags" hint="Comma-separated list of agent tags">
                <input
                  className={inputCls}
                  value={tagsInput}
                  onChange={e => setTagsInput(e.target.value)}
                  placeholder="gpu, windows"
                />
              </FormRow>
              <FormRow label="Enabled">
                <label className="flex items-center gap-2 cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={form.enabled ?? true}
                    onChange={e => setForm(prev => ({ ...prev, enabled: e.target.checked }))}
                    className="h-4 w-4 rounded border-th-border text-blue-600"
                  />
                  <span className="text-sm text-th-text">Active</span>
                </label>
              </FormRow>
            </div>
            <div className="px-6 py-4 border-t border-th-border flex justify-end gap-3">
              <button
                onClick={() => setShowForm(false)}
                className="rounded border border-th-border px-4 py-2 text-sm text-th-text hover:bg-th-surface-muted"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                disabled={saving}
                className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
              >
                {saving ? 'Saving…' : editingId ? 'Save Changes' : 'Create Profile'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

const inputCls =
  'w-full rounded border border-th-input-border bg-th-input px-3 py-1.5 text-sm text-th-text placeholder:text-th-text-subtle focus:outline-none focus:ring-1 focus:ring-blue-500'

function FormRow({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <label className="block text-sm font-medium text-th-text">{label}</label>
      {children}
      {hint && <p className="text-xs text-th-text-muted">{hint}</p>}
    </div>
  )
}
