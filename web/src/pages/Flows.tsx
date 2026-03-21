import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import type { Flow } from '../types/flow'

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

export default function Flows() {
  const navigate = useNavigate()
  const [flows, setFlows] = useState<Flow[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [duplicatingId, setDuplicatingId] = useState<string | null>(null)

  const load = useCallback(() => {
    setLoading(true)
    setError('')
    api.listFlows()
      .then(setFlows)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed to load flows'))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const handleDelete = async (f: Flow) => {
    if (!confirm(`Delete flow "${f.name}"? This cannot be undone.`)) return
    setDeletingId(f.id)
    try {
      await api.deleteFlow(f.id)
      setFlows(prev => prev.filter(fl => fl.id !== f.id))
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    } finally {
      setDeletingId(null)
    }
  }

  const handleDuplicate = async (f: Flow) => {
    setDuplicatingId(f.id)
    try {
      const created = await api.createFlow({
        name: `${f.name} (copy)`,
        description: f.description,
        nodes: f.nodes,
        edges: f.edges,
      })
      setFlows(prev => [created, ...prev])
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Duplicate failed')
    } finally {
      setDuplicatingId(null)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16 text-th-text-muted text-sm">
        Loading flows…
      </div>
    )
  }

  return (
    <div className="space-y-4 max-w-6xl">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-th-text">Pipeline Flows</h1>
          <p className="text-sm text-th-text-muted mt-0.5">
            Visual node-based encoding pipelines. Drag nodes to build workflows.
          </p>
        </div>
        <button
          onClick={() => navigate('/flows/editor')}
          className="px-4 py-2 rounded text-sm font-medium bg-blue-600 text-white hover:bg-blue-700 transition-colors flex items-center gap-2"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
          </svg>
          Create New Flow
        </button>
      </div>

      {error && (
        <div className="bg-red-50 border border-red-200 rounded p-3 text-sm text-red-700">
          {error}
        </div>
      )}

      {/* Empty state */}
      {flows.length === 0 && !loading && (
        <div className="bg-th-surface rounded-lg border border-th-border p-12 text-center">
          <div className="text-4xl mb-3">🔀</div>
          <p className="text-th-text font-medium mb-1">No flows yet</p>
          <p className="text-th-text-muted text-sm mb-4">
            Create a flow to build a visual encoding pipeline.
          </p>
          <button
            onClick={() => navigate('/flows/editor')}
            className="px-4 py-2 rounded text-sm font-medium bg-blue-600 text-white hover:bg-blue-700 transition-colors"
          >
            Create Your First Flow
          </button>
        </div>
      )}

      {/* Flow table */}
      {flows.length > 0 && (
        <div className="bg-th-surface rounded-lg border border-th-border overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-th-border bg-th-surface-muted">
                <th className="text-left px-4 py-2.5 text-xs font-semibold text-th-text-muted uppercase tracking-wide">
                  Name
                </th>
                <th className="text-left px-4 py-2.5 text-xs font-semibold text-th-text-muted uppercase tracking-wide hidden md:table-cell">
                  Description
                </th>
                <th className="text-center px-4 py-2.5 text-xs font-semibold text-th-text-muted uppercase tracking-wide">
                  Nodes
                </th>
                <th className="text-center px-4 py-2.5 text-xs font-semibold text-th-text-muted uppercase tracking-wide">
                  Edges
                </th>
                <th className="text-left px-4 py-2.5 text-xs font-semibold text-th-text-muted uppercase tracking-wide hidden lg:table-cell">
                  Updated
                </th>
                <th className="px-4 py-2.5" />
              </tr>
            </thead>
            <tbody>
              {flows.map(f => (
                <tr
                  key={f.id}
                  className="border-b border-th-border last:border-b-0 hover:bg-th-surface-muted transition-colors cursor-pointer"
                  onClick={() => navigate(`/flows/editor/${f.id}`)}
                >
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <span className="text-base">🔀</span>
                      <span className="font-medium text-th-text">{f.name}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-th-text-muted hidden md:table-cell max-w-xs truncate">
                    {f.description || <span className="text-th-text-subtle italic">No description</span>}
                  </td>
                  <td className="px-4 py-3 text-center">
                    <span className="inline-flex items-center justify-center min-w-[2rem] px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-700">
                      {Array.isArray(f.nodes) ? f.nodes.length : 0}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-center">
                    <span className="inline-flex items-center justify-center min-w-[2rem] px-2 py-0.5 rounded-full text-xs font-medium bg-th-surface-muted text-th-text-muted">
                      {Array.isArray(f.edges) ? f.edges.length : 0}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-th-text-muted text-xs hidden lg:table-cell">
                    {fmtDate(f.updated_at)}
                  </td>
                  <td className="px-4 py-3">
                    <div
                      className="flex items-center gap-1 justify-end"
                      onClick={e => e.stopPropagation()}
                    >
                      {/* Duplicate */}
                      <button
                        onClick={() => handleDuplicate(f)}
                        disabled={duplicatingId === f.id}
                        className="px-2 py-1 rounded text-xs text-th-text-muted hover:bg-th-surface-muted hover:text-th-text transition-colors disabled:opacity-50"
                        title="Duplicate flow"
                      >
                        {duplicatingId === f.id ? '…' : 'Copy'}
                      </button>
                      {/* Edit */}
                      <button
                        onClick={() => navigate(`/flows/editor/${f.id}`)}
                        className="px-2 py-1 rounded text-xs text-blue-600 hover:bg-blue-50 transition-colors"
                        title="Edit flow"
                      >
                        Edit
                      </button>
                      {/* Delete */}
                      <button
                        onClick={() => handleDelete(f)}
                        disabled={deletingId === f.id}
                        className="px-2 py-1 rounded text-xs text-red-600 hover:bg-red-50 transition-colors disabled:opacity-50"
                        title="Delete flow"
                      >
                        {deletingId === f.id ? '…' : 'Delete'}
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
