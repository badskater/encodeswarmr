import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { AgentPool, Agent } from '../../types'
import { useAutoRefresh } from '../../hooks/useAutoRefresh'

const PRESET_COLORS = [
  '#6366f1', '#8b5cf6', '#ec4899', '#ef4444',
  '#f97316', '#eab308', '#22c55e', '#14b8a6',
  '#3b82f6', '#06b6d4', '#64748b', '#78716c',
]

function ColorPicker({ value, onChange }: { value: string; onChange: (c: string) => void }) {
  return (
    <div className="flex flex-wrap gap-2 mt-1">
      {PRESET_COLORS.map(c => (
        <button
          key={c}
          type="button"
          onClick={() => onChange(c)}
          className="w-7 h-7 rounded-full border-2 transition-transform hover:scale-110"
          style={{
            backgroundColor: c,
            borderColor: value === c ? 'white' : 'transparent',
            boxShadow: value === c ? `0 0 0 2px ${c}` : 'none',
          }}
          title={c}
        />
      ))}
    </div>
  )
}

interface PoolFormProps {
  initial?: AgentPool
  onSave: (data: { name: string; description: string; tags: string[]; color: string }) => Promise<void>
  onCancel: () => void
}

function PoolForm({ initial, onSave, onCancel }: PoolFormProps) {
  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [tagsInput, setTagsInput] = useState(initial?.tags?.join(', ') ?? '')
  const [color, setColor] = useState(initial?.color ?? '#6366f1')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) { setError('Name is required'); return }
    setSaving(true)
    setError('')
    try {
      const tags = tagsInput.split(',').map(t => t.trim()).filter(Boolean)
      await onSave({ name: name.trim(), description: description.trim(), tags, color })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      {error && <p className="text-red-600 text-sm">{error}</p>}
      <div>
        <label className="block text-xs text-th-text-muted mb-1">Name *</label>
        <input
          value={name}
          onChange={e => setName(e.target.value)}
          className="w-full bg-th-input-bg border border-th-input-border rounded px-3 py-1.5 text-sm text-th-text"
          placeholder="e.g. NVENC Pool"
        />
      </div>
      <div>
        <label className="block text-xs text-th-text-muted mb-1">Description</label>
        <input
          value={description}
          onChange={e => setDescription(e.target.value)}
          className="w-full bg-th-input-bg border border-th-input-border rounded px-3 py-1.5 text-sm text-th-text"
          placeholder="Optional description"
        />
      </div>
      <div>
        <label className="block text-xs text-th-text-muted mb-1">Tags (comma-separated)</label>
        <input
          value={tagsInput}
          onChange={e => setTagsInput(e.target.value)}
          className="w-full bg-th-input-bg border border-th-input-border rounded px-3 py-1.5 text-sm text-th-text"
          placeholder="e.g. nvenc, gpu, h264"
        />
        <p className="text-xs text-th-text-subtle mt-1">Agents assigned to this pool will have these tags added to them.</p>
      </div>
      <div>
        <label className="block text-xs text-th-text-muted mb-1">Color</label>
        <ColorPicker value={color} onChange={setColor} />
      </div>
      <div className="flex gap-2 pt-1">
        <button
          type="submit"
          disabled={saving}
          className="px-4 py-1.5 bg-blue-600 text-white rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
        >
          {saving ? 'Saving…' : initial ? 'Update Pool' : 'Create Pool'}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="px-4 py-1.5 border border-th-border text-th-text-secondary rounded text-sm hover:bg-th-surface-muted"
        >
          Cancel
        </button>
      </div>
    </form>
  )
}

export default function AgentPools() {
  const [pools, setPools] = useState<AgentPool[]>([])
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<string | null>(null)
  const [deleting, setDeleting] = useState<string | null>(null)
  const [assignModal, setAssignModal] = useState<string | null>(null)
  const [assignTarget, setAssignTarget] = useState('')

  const load = useCallback(async () => {
    try {
      const [p, a] = await Promise.all([api.listAgentPools(), api.listAgents()])
      setPools(p)
      setAgents(a)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])
  useAutoRefresh(load)

  const handleCreate = async (data: { name: string; description: string; tags: string[]; color: string }) => {
    await api.createAgentPool(data)
    setCreating(false)
    load()
  }

  const handleUpdate = async (id: string, data: { name: string; description: string; tags: string[]; color: string }) => {
    await api.updateAgentPool(id, data)
    setEditing(null)
    load()
  }

  const handleDelete = async (id: string) => {
    setDeleting(id)
    try {
      await api.deleteAgentPool(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    } finally {
      setDeleting(null)
    }
  }

  const handleAssign = async () => {
    if (!assignModal || !assignTarget) return
    try {
      await api.assignAgentToPool(assignTarget, assignModal)
      setAssignModal(null)
      setAssignTarget('')
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to assign agent')
    }
  }

  const getPoolMembers = (pool: AgentPool) => {
    return agents.filter(a => pool.tags.length > 0 && pool.tags.every(t => a.tags.includes(t)))
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Agent Pools</h1>
        {!creating && (
          <button
            onClick={() => setCreating(true)}
            className="px-4 py-1.5 bg-blue-600 text-white rounded text-sm font-medium hover:bg-blue-700"
          >
            New Pool
          </button>
        )}
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {creating && (
        <div className="bg-th-surface rounded-lg shadow p-4">
          <h2 className="text-sm font-semibold text-th-text-secondary mb-3">Create Pool</h2>
          <PoolForm
            onSave={handleCreate}
            onCancel={() => setCreating(false)}
          />
        </div>
      )}

      {pools.length === 0 && !creating && (
        <div className="bg-th-surface rounded-lg shadow px-6 py-8 text-center text-th-text-subtle">
          No agent pools configured. Create a pool to group agents by tags.
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {pools.map(pool => {
          const members = getPoolMembers(pool)
          return (
            <div key={pool.id} className="bg-th-surface rounded-lg shadow">
              {editing === pool.id ? (
                <div className="p-4">
                  <h3 className="text-sm font-semibold text-th-text-secondary mb-3">Edit Pool</h3>
                  <PoolForm
                    initial={pool}
                    onSave={data => handleUpdate(pool.id, data)}
                    onCancel={() => setEditing(null)}
                  />
                </div>
              ) : (
                <>
                  <div className="px-4 py-3 flex items-center gap-3 border-b border-th-border-subtle">
                    <div
                      className="w-4 h-4 rounded-full shrink-0"
                      style={{ backgroundColor: pool.color }}
                    />
                    <div className="flex-1 min-w-0">
                      <p className="font-medium text-th-text truncate">{pool.name}</p>
                      {pool.description && (
                        <p className="text-xs text-th-text-muted truncate">{pool.description}</p>
                      )}
                    </div>
                    <span className="text-xs text-th-text-subtle whitespace-nowrap">{members.length} agent{members.length !== 1 ? 's' : ''}</span>
                  </div>

                  <div className="px-4 py-2">
                    {pool.tags.length > 0 ? (
                      <div className="flex flex-wrap gap-1 mb-2">
                        {pool.tags.map(t => (
                          <span
                            key={t}
                            className="px-1.5 py-0.5 rounded text-xs font-mono"
                            style={{ backgroundColor: pool.color + '22', color: pool.color }}
                          >
                            {t}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <p className="text-xs text-th-text-subtle mb-2">No tags</p>
                    )}

                    {members.length > 0 && (
                      <div className="text-xs text-th-text-muted mb-2">
                        <span className="font-medium">Members: </span>
                        {members.map(a => a.name).join(', ')}
                      </div>
                    )}

                    <div className="flex gap-2 flex-wrap mt-2">
                      <button
                        onClick={() => { setAssignModal(pool.id); setAssignTarget('') }}
                        className="text-xs px-2 py-1 rounded border border-th-input-border text-th-text-muted hover:bg-th-surface-muted"
                      >
                        Assign Agent
                      </button>
                      <button
                        onClick={() => setEditing(pool.id)}
                        className="text-xs px-2 py-1 rounded border border-th-input-border text-th-text-muted hover:bg-th-surface-muted"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDelete(pool.id)}
                        disabled={deleting === pool.id}
                        className="text-xs px-2 py-1 rounded text-red-600 border border-red-200 hover:bg-red-50 disabled:opacity-50"
                      >
                        {deleting === pool.id ? 'Deleting…' : 'Delete'}
                      </button>
                    </div>
                  </div>
                </>
              )}
            </div>
          )
        })}
      </div>

      {/* Assign agent modal */}
      {assignModal && (
        <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
          <div className="bg-th-surface rounded-lg shadow-xl p-6 w-full max-w-sm mx-4">
            <h2 className="text-base font-semibold text-th-text mb-3">Assign Agent to Pool</h2>
            <p className="text-sm text-th-text-muted mb-3">
              Select an agent to assign to <strong>{pools.find(p => p.id === assignModal)?.name}</strong>.
              The pool's tags will be added to the agent.
            </p>
            <select
              value={assignTarget}
              onChange={e => setAssignTarget(e.target.value)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-3 py-1.5 text-sm text-th-text mb-4"
            >
              <option value="">Select agent…</option>
              {agents.map(a => (
                <option key={a.id} value={a.id}>{a.name} ({a.status})</option>
              ))}
            </select>
            <div className="flex gap-2">
              <button
                onClick={handleAssign}
                disabled={!assignTarget}
                className="flex-1 px-4 py-1.5 bg-blue-600 text-white rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
              >
                Assign
              </button>
              <button
                onClick={() => { setAssignModal(null); setAssignTarget('') }}
                className="flex-1 px-4 py-1.5 border border-th-border text-th-text-secondary rounded text-sm hover:bg-th-surface-muted"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
