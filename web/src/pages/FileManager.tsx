import { useState, useEffect, useCallback } from 'react'
import * as api from '../api/client'
import type { FileEntry, FileInfo } from '../types'

function fmtSize(n: number) {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + ' GB'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + ' MB'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + ' KB'
  return n + ' B'
}

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

function fileIcon(entry: FileEntry) {
  if (entry.is_dir) return '📁'
  if (entry.is_video) return '🎬'
  const ext = entry.ext?.toLowerCase()
  if (['.srt', '.ass', '.vtt'].includes(ext)) return '💬'
  if (['.mp3', '.flac', '.opus', '.aac', '.wav'].includes(ext)) return '🎵'
  return '📄'
}

interface BreadcrumbPart {
  label: string
  path: string
}

function buildBreadcrumbs(path: string): BreadcrumbPart[] {
  const parts = path.split('/').filter(Boolean)
  const crumbs: BreadcrumbPart[] = [{ label: '/', path: '/' }]
  let current = ''
  for (const part of parts) {
    current += '/' + part
    crumbs.push({ label: part, path: current })
  }
  return crumbs
}

export default function FileManager() {
  const [currentPath, setCurrentPath] = useState('/')
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<FileEntry | null>(null)
  const [fileInfo, setFileInfo] = useState<FileInfo | null>(null)
  const [infoLoading, setInfoLoading] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<FileEntry | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [moveTarget, setMoveTarget] = useState<FileEntry | null>(null)
  const [moveDest, setMoveDest] = useState('')
  const [moving, setMoving] = useState(false)

  const navigate = useCallback(async (path: string) => {
    setLoading(true)
    setError('')
    setSelected(null)
    setFileInfo(null)
    try {
      const result = await api.browseFiles(path)
      setCurrentPath(result.path)
      // Sort: directories first, then files, both alphabetically.
      const sorted = [...result.entries].sort((a, b) => {
        if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
        return a.name.localeCompare(b.name)
      })
      setEntries(sorted)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to browse directory')
    } finally {
      setLoading(false)
    }
  }, [])

  // Load the root on mount; errors are shown inline.
  useEffect(() => {
    navigate('/').catch(() => {
      // Root may not be allowed — show error inline, don't crash.
    })
  }, [navigate])

  const handleEntryClick = async (entry: FileEntry) => {
    if (entry.is_dir) {
      navigate(entry.path)
      return
    }
    setSelected(entry)
    setFileInfo(null)
    setInfoLoading(true)
    try {
      const info = await api.getFileInfo(entry.path)
      setFileInfo(info)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to get file info')
    } finally {
      setInfoLoading(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      await api.deleteFile(deleteTarget.path)
      setDeleteTarget(null)
      if (selected?.path === deleteTarget.path) {
        setSelected(null)
        setFileInfo(null)
      }
      navigate(currentPath)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Delete failed')
    } finally {
      setDeleting(false)
    }
  }

  const handleMove = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!moveTarget || !moveDest.trim()) return
    setMoving(true)
    try {
      await api.moveFile(moveTarget.path, moveDest.trim())
      setMoveTarget(null)
      setMoveDest('')
      navigate(currentPath)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Move failed')
    } finally {
      setMoving(false)
    }
  }

  const handleCreateSource = async (entry: FileEntry) => {
    try {
      await api.createSource({ path: entry.path, name: entry.name })
      setError('')
      alert(`Source created from ${entry.name}`)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create source')
    }
  }

  const crumbs = buildBreadcrumbs(currentPath)

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">File Manager</h1>
      </div>

      {error && (
        <p className="text-red-600 text-sm">{error}</p>
      )}

      {/* Breadcrumb */}
      <nav className="flex items-center gap-1 text-sm text-th-text-muted flex-wrap">
        {crumbs.map((crumb, i) => (
          <span key={crumb.path} className="flex items-center gap-1">
            {i > 0 && <span className="text-th-border">/</span>}
            <button
              onClick={() => navigate(crumb.path)}
              className="hover:text-th-text hover:underline"
            >
              {crumb.label}
            </button>
          </span>
        ))}
      </nav>

      <div className="flex gap-4">
        {/* File list */}
        <div className="flex-1 bg-th-surface rounded-lg shadow overflow-hidden">
          {loading ? (
            <p className="p-4 text-th-text-muted text-sm">Loading…</p>
          ) : (
            <table className="min-w-full divide-y divide-th-border text-sm">
              <thead className="bg-th-surface-muted">
                <tr>
                  {['Name', 'Size', 'Modified', ''].map(h => (
                    <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-th-border-subtle">
                {/* Parent directory ".." */}
                {currentPath !== '/' && (
                  <tr
                    className="hover:bg-th-surface-muted cursor-pointer"
                    onClick={() => {
                      const parent = currentPath.split('/').slice(0, -1).join('/') || '/'
                      navigate(parent)
                    }}
                  >
                    <td className="px-4 py-2 text-th-text-muted" colSpan={4}>📁 ..</td>
                  </tr>
                )}
                {entries.map(entry => (
                  <tr
                    key={entry.path}
                    onClick={() => handleEntryClick(entry)}
                    className={`cursor-pointer ${selected?.path === entry.path ? 'bg-blue-50 dark:bg-blue-900/20' : 'hover:bg-th-surface-muted'}`}
                  >
                    <td className="px-4 py-2 text-th-text font-medium">
                      {fileIcon(entry)} {entry.name}
                    </td>
                    <td className="px-4 py-2 text-th-text-secondary whitespace-nowrap">
                      {entry.is_dir ? '—' : fmtSize(entry.size)}
                    </td>
                    <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">
                      {fmtDate(entry.mod_time)}
                    </td>
                    <td className="px-4 py-2">
                      {!entry.is_dir && (
                        <div className="flex items-center gap-1" onClick={e => e.stopPropagation()}>
                          <a
                            href={api.fileDownloadURL(entry.path)}
                            download={entry.name}
                            className="text-xs px-2 py-1 rounded text-blue-600 hover:bg-th-surface-muted"
                            title="Download"
                          >
                            Download
                          </a>
                          <button
                            onClick={() => { setMoveTarget(entry); setMoveDest('') }}
                            className="text-xs px-2 py-1 rounded text-th-text-secondary hover:bg-th-surface-muted"
                            title="Move"
                          >
                            Move
                          </button>
                          {entry.is_video && (
                            <button
                              onClick={() => handleCreateSource(entry)}
                              className="text-xs px-2 py-1 rounded text-green-600 hover:bg-th-surface-muted"
                              title="Create source"
                            >
                              + Source
                            </button>
                          )}
                          <button
                            onClick={() => setDeleteTarget(entry)}
                            className="text-xs px-2 py-1 rounded text-red-600 hover:bg-th-surface-muted"
                            title="Delete"
                          >
                            Delete
                          </button>
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
                {entries.length === 0 && !loading && (
                  <tr><td colSpan={4} className="px-4 py-4 text-center text-th-text-subtle">Directory is empty</td></tr>
                )}
              </tbody>
            </table>
          )}
        </div>

        {/* File detail panel */}
        {selected && (
          <div className="w-72 bg-th-surface rounded-lg shadow p-4 space-y-3 flex-shrink-0">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-semibold text-th-text truncate">{selected.name}</h2>
              <button
                onClick={() => { setSelected(null); setFileInfo(null) }}
                className="text-th-text-muted hover:text-th-text text-lg leading-none"
              >
                ×
              </button>
            </div>

            <div className="space-y-1 text-xs text-th-text-muted">
              <div><span className="font-medium">Path:</span> <span className="text-th-text-secondary break-all">{selected.path}</span></div>
              <div><span className="font-medium">Size:</span> <span className="text-th-text-secondary">{fmtSize(selected.size)}</span></div>
              <div><span className="font-medium">Modified:</span> <span className="text-th-text-secondary">{fmtDate(selected.mod_time)}</span></div>
              {selected.ext && <div><span className="font-medium">Extension:</span> <span className="text-th-text-secondary">{selected.ext}</span></div>}
            </div>

            {infoLoading && <p className="text-xs text-th-text-muted">Probing…</p>}

            {fileInfo?.codec_info && (
              <div className="space-y-1">
                <h3 className="text-xs font-semibold text-th-text-secondary">Codec Info</h3>
                <pre className="text-xs text-th-text-muted bg-th-surface-muted p-2 rounded overflow-auto max-h-48">
                  {JSON.stringify(fileInfo.codec_info, null, 2)}
                </pre>
              </div>
            )}

            <div className="flex flex-col gap-2 pt-2">
              {selected.is_video && (
                <button
                  onClick={() => handleCreateSource(selected)}
                  className="w-full bg-green-600 text-white text-xs py-1.5 rounded hover:bg-green-700"
                >
                  Create Source from File
                </button>
              )}
              <a
                href={api.fileDownloadURL(selected.path)}
                download={selected.name}
                className="w-full text-center bg-blue-600 text-white text-xs py-1.5 rounded hover:bg-blue-700"
              >
                Download
              </a>
              <button
                onClick={() => { setMoveTarget(selected); setMoveDest('') }}
                className="w-full border border-th-border text-th-text text-xs py-1.5 rounded hover:bg-th-surface-muted"
              >
                Move
              </button>
              <button
                onClick={() => setDeleteTarget(selected)}
                className="w-full border border-red-300 text-red-600 text-xs py-1.5 rounded hover:bg-red-50"
              >
                Delete
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Delete confirmation modal */}
      {deleteTarget && (
        <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
          <div className="bg-th-surface rounded-lg shadow-lg p-6 max-w-md w-full mx-4 space-y-4">
            <h2 className="text-lg font-semibold text-th-text">Confirm Delete</h2>
            <p className="text-sm text-th-text-muted">
              Are you sure you want to permanently delete{' '}
              <span className="font-medium text-th-text">{deleteTarget.name}</span>?
              This action cannot be undone.
            </p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setDeleteTarget(null)}
                className="px-4 py-2 text-sm border border-th-border rounded hover:bg-th-surface-muted"
              >
                Cancel
              </button>
              <button
                onClick={handleDelete}
                disabled={deleting}
                className="px-4 py-2 text-sm bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50"
              >
                {deleting ? 'Deleting…' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Move modal */}
      {moveTarget && (
        <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50">
          <div className="bg-th-surface rounded-lg shadow-lg p-6 max-w-md w-full mx-4 space-y-4">
            <h2 className="text-lg font-semibold text-th-text">Move File</h2>
            <form onSubmit={handleMove} className="space-y-3">
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Source</label>
                <p className="text-sm text-th-text font-mono break-all">{moveTarget.path}</p>
              </div>
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Destination *</label>
                <input
                  type="text"
                  value={moveDest}
                  onChange={e => setMoveDest(e.target.value)}
                  required
                  placeholder="/mnt/nas/output/movie.mkv"
                  className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                />
              </div>
              <div className="flex gap-3 justify-end">
                <button
                  type="button"
                  onClick={() => { setMoveTarget(null); setMoveDest('') }}
                  className="px-4 py-2 text-sm border border-th-border rounded hover:bg-th-surface-muted"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={moving}
                  className="px-4 py-2 text-sm bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
                >
                  {moving ? 'Moving…' : 'Move'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
