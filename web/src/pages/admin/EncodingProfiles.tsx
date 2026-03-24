import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'

export default function EncodingProfiles() {
  const [profiles, setProfiles] = useState<api.EncodingProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editing, setEditing] = useState<api.EncodingProfile | null>(null)
  const [creating, setCreating] = useState(false)

  const load = useCallback(async () => {
    try {
      const data = await api.listEncodingProfiles()
      setProfiles(data)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load profiles')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this encoding profile?')) return
    try {
      await api.deleteEncodingProfile(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete profile')
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-6 max-w-4xl">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-th-text">Encoding Profiles</h1>
          <p className="text-sm text-th-text-muted mt-0.5">
            Reusable encoding presets that can be loaded when creating jobs.
          </p>
        </div>
        <button
          onClick={() => setCreating(true)}
          className="rounded bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700"
        >
          + New Profile
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {(creating || editing) && (
        <ProfileForm
          profile={editing}
          onSave={() => { setCreating(false); setEditing(null); load() }}
          onCancel={() => { setCreating(false); setEditing(null) }}
        />
      )}

      {profiles.length === 0 ? (
        <p className="text-sm text-th-text-muted">No encoding profiles configured yet.</p>
      ) : (
        <div className="space-y-3">
          {profiles.map(p => (
            <div key={p.id} className="bg-th-surface rounded-lg shadow p-4 flex items-start justify-between gap-4">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <p className="text-sm font-semibold text-th-text">{p.name}</p>
                  <span className="text-xs text-th-text-muted bg-th-surface-muted rounded px-1.5 py-0.5">
                    {p.container}
                  </span>
                </div>
                {p.description && (
                  <p className="text-xs text-th-text-muted mt-0.5">{p.description}</p>
                )}
                <p className="text-xs text-th-text-subtle mt-1">
                  Created by {p.created_by || 'system'} · {new Date(p.created_at).toLocaleDateString()}
                </p>
              </div>
              <div className="flex gap-2 shrink-0">
                <button
                  onClick={() => { setEditing(p); setCreating(false) }}
                  className="text-sm text-blue-600 hover:underline"
                >
                  Edit
                </button>
                <button
                  onClick={() => handleDelete(p.id)}
                  className="text-sm text-red-500 hover:underline"
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function ProfileForm({
  profile,
  onSave,
  onCancel,
}: {
  profile: api.EncodingProfile | null
  onSave: () => void
  onCancel: () => void
}) {
  const [name, setName] = useState(profile?.name ?? '')
  const [description, setDescription] = useState(profile?.description ?? '')
  const [container, setContainer] = useState(profile?.container ?? 'mkv')
  const [settingsText, setSettingsText] = useState(
    profile?.settings ? JSON.stringify(profile.settings, null, 2) : '{}'
  )
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState('')

  const handleSubmit = async () => {
    setSaving(true)
    setFormError('')
    try {
      let settings: Record<string, unknown>
      try {
        settings = JSON.parse(settingsText)
      } catch {
        setFormError('Settings must be valid JSON.')
        setSaving(false)
        return
      }

      if (profile) {
        await api.updateEncodingProfile(profile.id, { name, description, container, settings })
      } else {
        await api.createEncodingProfile({ name, description, container, settings })
      }
      onSave()
    } catch (e: unknown) {
      setFormError(e instanceof Error ? e.message : 'Failed to save profile')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="bg-th-surface rounded-lg shadow p-5 space-y-4 border border-th-border">
      <h2 className="text-sm font-semibold text-th-text">
        {profile ? 'Edit Profile' : 'New Encoding Profile'}
      </h2>
      {formError && <p className="text-red-600 text-sm">{formError}</p>}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="space-y-1">
          <label className="block text-sm font-medium text-th-text">Name *</label>
          <input
            type="text"
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="e.g. 1080p HEVC High Quality"
            className="w-full rounded border border-th-border bg-th-input px-3 py-1.5 text-sm text-th-text focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>
        <div className="space-y-1">
          <label className="block text-sm font-medium text-th-text">Container</label>
          <select
            value={container}
            onChange={e => setContainer(e.target.value)}
            className="w-full rounded border border-th-border bg-th-input px-3 py-1.5 text-sm text-th-text focus:outline-none focus:ring-1 focus:ring-blue-500"
          >
            <option value="mkv">MKV</option>
            <option value="mp4">MP4</option>
            <option value="ts">TS</option>
            <option value="avi">AVI</option>
          </select>
        </div>
      </div>

      <div className="space-y-1">
        <label className="block text-sm font-medium text-th-text">Description</label>
        <input
          type="text"
          value={description}
          onChange={e => setDescription(e.target.value)}
          placeholder="Optional description"
          className="w-full rounded border border-th-border bg-th-input px-3 py-1.5 text-sm text-th-text focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
      </div>

      <div className="space-y-1">
        <label className="block text-sm font-medium text-th-text">
          Settings <span className="text-th-text-muted font-normal">(JSON)</span>
        </label>
        <textarea
          value={settingsText}
          onChange={e => setSettingsText(e.target.value)}
          rows={6}
          className="w-full rounded border border-th-border bg-th-input px-3 py-1.5 text-sm text-th-text font-mono focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        <p className="text-xs text-th-text-muted">
          JSON object with template_id, extra_vars, etc. matched to EncodeConfig fields.
        </p>
      </div>

      <div className="flex justify-end gap-2">
        <button
          onClick={onCancel}
          className="rounded border border-th-border px-3 py-1.5 text-sm text-th-text hover:bg-th-surface-muted"
        >
          Cancel
        </button>
        <button
          onClick={handleSubmit}
          disabled={saving || !name}
          className="rounded bg-blue-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {saving ? 'Saving…' : profile ? 'Save Changes' : 'Create Profile'}
        </button>
      </div>
    </div>
  )
}
