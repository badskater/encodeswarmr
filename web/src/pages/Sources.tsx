import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import type { Source } from '../types'
import StatusBadge from '../components/StatusBadge'
import { useAutoRefresh } from '../hooks/useAutoRefresh'

function fmtBytes(n: number) {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + ' GB'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + ' MB'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + ' KB'
  return n + ' B'
}

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

function fmtDuration(sec: number | null) {
  if (sec == null) return '—'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m ${s}s`
}

export default function Sources() {
  const [sources, setSources] = useState<Source[]>([])
  const [nextCursor, setNextCursor] = useState<string | undefined>()
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [formPath, setFormPath] = useState('')
  const [formName, setFormName] = useState('')
  const [formCloudURI, setFormCloudURI] = useState('')
  const [formSaving, setFormSaving] = useState(false)
  const [analyzingId, setAnalyzingId] = useState<string | null>(null)
  const [detectingHDRId, setDetectingHDRId] = useState<string | null>(null)
  const navigate = useNavigate()

  const load = useCallback(async () => {
    try {
      const result = await api.listSourcesPaged({})
      setSources(result.items)
      setNextCursor(result.nextCursor)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])
  useAutoRefresh(load)

  const handleLoadMore = async () => {
    if (!nextCursor) return
    setLoadingMore(true)
    try {
      const result = await api.listSourcesPaged({ cursor: nextCursor })
      setSources(prev => [...prev, ...result.items])
      setNextCursor(result.nextCursor)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load more')
    } finally {
      setLoadingMore(false)
    }
  }

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormSaving(true)
    try {
      const body: { path?: string; name?: string; cloud_uri?: string } = {}
      if (formCloudURI.trim()) {
        body.cloud_uri = formCloudURI.trim()
      } else {
        body.path = formPath
      }
      if (formName) body.name = formName
      await api.createSource(body)
      setShowForm(false)
      setFormPath('')
      setFormName('')
      setFormCloudURI('')
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to register source')
    } finally {
      setFormSaving(false)
    }
  }

  const handleAnalyze = async (id: string, ev: React.MouseEvent) => {
    ev.stopPropagation()
    setAnalyzingId(id)
    try {
      await api.analyzeSource(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to trigger analysis')
    } finally {
      setAnalyzingId(null)
    }
  }

  const handleHDRDetect = async (id: string, ev: React.MouseEvent) => {
    ev.stopPropagation()
    setDetectingHDRId(id)
    try {
      await api.hdrDetectSource(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to trigger HDR detection')
    } finally {
      setDetectingHDRId(null)
    }
  }

  function hdrLabel(hdrType: string, dvProfile: number): string {
    if (hdrType === 'dolby_vision') return dvProfile > 0 ? `DV P${dvProfile}` : 'Dolby Vision'
    if (hdrType === 'hdr10plus') return 'HDR10+'
    if (hdrType === 'hdr10') return 'HDR10'
    if (hdrType === 'hlg') return 'HLG'
    return '—'
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Sources</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
        >
          {showForm ? 'Cancel' : 'Register Source'}
        </button>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {showForm && (
        <form onSubmit={handleRegister} className="bg-th-surface rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-th-text-secondary">Register Source</h2>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">
              UNC Path {!formCloudURI.trim() && <span className="text-red-500">*</span>}
            </label>
            <input
              value={formPath}
              onChange={e => setFormPath(e.target.value)}
              placeholder="\\server\share\videos\file.mkv"
              required={!formCloudURI.trim()}
              disabled={!!formCloudURI.trim()}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm font-mono text-th-text disabled:opacity-40"
            />
          </div>
          <div className="flex items-center gap-2 text-xs text-th-text-muted">
            <span className="border-t border-th-border flex-1" />
            <span>or</span>
            <span className="border-t border-th-border flex-1" />
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">
              Cloud URI {!formPath.trim() && <span className="text-blue-500">(optional)</span>}
            </label>
            <input
              value={formCloudURI}
              onChange={e => setFormCloudURI(e.target.value)}
              placeholder="s3://bucket/path/to/video.mkv"
              disabled={!!formPath.trim()}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm font-mono text-th-text disabled:opacity-40"
            />
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Name (optional)</label>
            <input
              value={formName}
              onChange={e => setFormName(e.target.value)}
              placeholder="Filename override"
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
            />
          </div>
          <button type="submit" disabled={formSaving}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
            {formSaving ? 'Registering…' : 'Register'}
          </button>
        </form>
      )}

      <div className="bg-th-surface rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-th-border text-sm">
          <thead className="bg-th-surface-muted">
            <tr>
              {['Filename', 'Path', 'Size', 'Duration', 'VMAF', 'HDR', 'State', 'Created', ''].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-th-border-subtle">
            {sources.map(s => (
              <tr
                key={s.id}
                onClick={() => navigate(`/sources/${s.id}`)}
                className="hover:bg-th-surface-muted cursor-pointer"
              >
                <td className="px-4 py-2 font-medium text-th-text">{s.filename}</td>
                <td className="px-4 py-2 text-th-text-muted max-w-xs truncate">{s.path}</td>
                <td className="px-4 py-2 text-th-text-secondary whitespace-nowrap">{fmtBytes(s.size_bytes)}</td>
                <td className="px-4 py-2 text-th-text-secondary whitespace-nowrap">{fmtDuration(s.duration_sec)}</td>
                <td className="px-4 py-2 text-th-text-secondary">
                  {s.vmaf_score != null ? s.vmaf_score.toFixed(1) : '—'}
                </td>
                <td className="px-4 py-2 text-th-text-secondary whitespace-nowrap">
                  {hdrLabel(s.hdr_type, s.dv_profile)}
                </td>
                <td className="px-4 py-2"><StatusBadge status={s.state} /></td>
                <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(s.created_at)}</td>
                <td className="px-4 py-2">
                  <div className="flex items-center gap-1">
                    <button
                      onClick={e => handleAnalyze(s.id, e)}
                      disabled={analyzingId === s.id}
                      className="text-xs px-2 py-1 rounded disabled:opacity-50"
                      style={{
                        backgroundColor: 'var(--th-badge-running-bg)',
                        color: 'var(--th-badge-running-text)',
                      }}
                    >
                      {analyzingId === s.id ? 'Queuing…' : 'Analyze'}
                    </button>
                    <button
                      onClick={e => handleHDRDetect(s.id, e)}
                      disabled={detectingHDRId === s.id}
                      className="text-xs px-2 py-1 rounded disabled:opacity-50"
                      style={{
                        backgroundColor: 'var(--th-badge-assigned-bg)',
                        color: 'var(--th-badge-assigned-text)',
                      }}
                    >
                      {detectingHDRId === s.id ? 'Queuing…' : 'HDR Detect'}
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {sources.length === 0 && (
              <tr><td colSpan={9} className="px-4 py-4 text-center text-th-text-subtle">No sources found</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {nextCursor && (
        <div className="text-center">
          <button
            onClick={handleLoadMore}
            disabled={loadingMore}
            className="px-4 py-2 text-sm text-th-text-secondary border border-th-border rounded hover:bg-th-surface-muted disabled:opacity-50"
          >
            {loadingMore ? 'Loading…' : 'Load more'}
          </button>
        </div>
      )}
    </div>
  )
}
