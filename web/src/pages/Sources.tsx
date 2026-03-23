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
  const [showBatchImport, setShowBatchImport] = useState(false)
  const [batchPattern, setBatchPattern] = useState('')
  const [batchRecursive, setBatchRecursive] = useState(false)
  const [batchAutoAnalyze, setBatchAutoAnalyze] = useState(true)
  const [batchSaving, setBatchSaving] = useState(false)
  const [batchResult, setBatchResult] = useState<{ imported: number } | null>(null)
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

  const handleBatchImport = async (e: React.FormEvent) => {
    e.preventDefault()
    setBatchSaving(true)
    setBatchResult(null)
    try {
      const result = await api.batchImportSources({
        path_pattern: batchPattern,
        recursive: batchRecursive,
        auto_analyze: batchAutoAnalyze,
      })
      setBatchResult({ imported: result.imported })
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Batch import failed')
    } finally {
      setBatchSaving(false)
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
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-2xl font-bold text-th-text">Sources</h1>
        <div className="flex gap-2">
          <button
            onClick={() => setShowBatchImport(!showBatchImport)}
            className="border border-th-border text-th-text px-3 py-1.5 rounded text-sm font-medium hover:bg-th-surface-muted shrink-0"
          >
            {showBatchImport ? 'Cancel Batch' : 'Batch Import'}
          </button>
          <button
            onClick={() => setShowForm(!showForm)}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 shrink-0"
          >
            {showForm ? 'Cancel' : 'Register Source'}
          </button>
        </div>
      </div>

      {showBatchImport && (
        <form onSubmit={handleBatchImport} className="bg-th-surface rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-th-text-secondary">Batch Import Sources</h2>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Path Pattern *</label>
            <input
              value={batchPattern}
              onChange={e => setBatchPattern(e.target.value)}
              required
              placeholder="\\NAS\media\*.mkv"
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
            />
            <p className="text-xs text-th-text-muted mt-0.5">
              Glob pattern resolved on the controller (UNC paths are translated via path mappings).
            </p>
          </div>
          <div className="flex gap-4 text-sm">
            <label className="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" checked={batchRecursive} onChange={e => setBatchRecursive(e.target.checked)} className="accent-blue-600" />
              Recursive
            </label>
            <label className="flex items-center gap-2 cursor-pointer">
              <input type="checkbox" checked={batchAutoAnalyze} onChange={e => setBatchAutoAnalyze(e.target.checked)} className="accent-blue-600" />
              Auto-analyze
            </label>
          </div>
          {batchResult && (
            <p className="text-green-600 text-sm">Imported {batchResult.imported} source(s).</p>
          )}
          <button
            type="submit"
            disabled={batchSaving}
            className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
          >
            {batchSaving ? 'Importing…' : 'Import'}
          </button>
        </form>
      )}

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

      {/* Desktop table */}
      <div className="hidden sm:block bg-th-surface rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-th-border text-sm">
          <thead className="bg-th-surface-muted">
            <tr>
              {['', 'Filename', 'Path', 'Size', 'Duration', 'VMAF', 'HDR', 'State', 'Created', ''].map(h => (
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
                <td className="px-2 py-2">
                  {s.thumbnails && s.thumbnails.length > 0 ? (
                    <img
                      src={s.thumbnails[0]}
                      alt="Preview"
                      className="w-16 h-9 object-cover rounded"
                      onError={e => { (e.target as HTMLImageElement).style.display = 'none' }}
                    />
                  ) : (
                    <div className="w-16 h-9 bg-th-surface-muted rounded flex items-center justify-center text-th-text-subtle text-xs">—</div>
                  )}
                </td>
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

      {/* Mobile card list */}
      <div className="sm:hidden space-y-3">
        {sources.map(s => (
          <div
            key={s.id}
            className="bg-th-surface rounded-lg shadow cursor-pointer hover:bg-th-surface-muted"
            onClick={() => navigate(`/sources/${s.id}`)}
          >
            <div className="px-4 py-3 border-b border-th-border-subtle">
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium text-th-text truncate">{s.filename}</span>
                <StatusBadge status={s.state} />
              </div>
              <p className="text-xs text-th-text-muted mt-0.5 truncate">{s.path}</p>
            </div>
            <div className="px-4 py-2 grid grid-cols-2 gap-1 text-xs">
              <div><span className="text-th-text-muted">Size: </span><span className="text-th-text-secondary">{fmtBytes(s.size_bytes)}</span></div>
              <div><span className="text-th-text-muted">Duration: </span><span className="text-th-text-secondary">{fmtDuration(s.duration_sec)}</span></div>
              <div><span className="text-th-text-muted">VMAF: </span><span className="text-th-text-secondary">{s.vmaf_score != null ? s.vmaf_score.toFixed(1) : '—'}</span></div>
              <div><span className="text-th-text-muted">HDR: </span><span className="text-th-text-secondary">{hdrLabel(s.hdr_type, s.dv_profile)}</span></div>
              <div className="col-span-2"><span className="text-th-text-muted">Created: </span><span className="text-th-text-secondary">{fmtDate(s.created_at)}</span></div>
            </div>
            <div className="px-4 py-2 flex gap-2 border-t border-th-border-subtle" onClick={e => e.stopPropagation()}>
              <button
                onClick={e => handleAnalyze(s.id, e)}
                disabled={analyzingId === s.id}
                className="text-xs px-2 py-1 rounded disabled:opacity-50"
                style={{ backgroundColor: 'var(--th-badge-running-bg)', color: 'var(--th-badge-running-text)' }}
              >
                {analyzingId === s.id ? 'Queuing…' : 'Analyze'}
              </button>
              <button
                onClick={e => handleHDRDetect(s.id, e)}
                disabled={detectingHDRId === s.id}
                className="text-xs px-2 py-1 rounded disabled:opacity-50"
                style={{ backgroundColor: 'var(--th-badge-assigned-bg)', color: 'var(--th-badge-assigned-text)' }}
              >
                {detectingHDRId === s.id ? 'Queuing…' : 'HDR Detect'}
              </button>
            </div>
          </div>
        ))}
        {sources.length === 0 && (
          <p className="text-center text-th-text-subtle text-sm py-8">No sources found</p>
        )}
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
