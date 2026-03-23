import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import { useAutoRefresh } from '../../hooks/useAutoRefresh'

interface WatchFolder {
  name: string
  path: string
  windows_path: string
  file_patterns: string[]
  poll_interval: string
  auto_analyze: boolean
  move_after_analysis?: string
  enabled: boolean
  last_scan?: string | null
}

export default function WatchFolders() {
  const [folders, setFolders] = useState<WatchFolder[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [scanning, setScanning] = useState<string | null>(null)
  const [toggling, setToggling] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const data = await api.listWatchFolders()
      setFolders(data)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load watch folders')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])
  useAutoRefresh(load)

  const handleToggle = async (name: string, currentEnabled: boolean) => {
    setToggling(name)
    try {
      await api.toggleWatchFolder(name, !currentEnabled)
      await load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to toggle watch folder')
    } finally {
      setToggling(null)
    }
  }

  const handleScan = async (name: string) => {
    setScanning(name)
    try {
      await api.scanWatchFolder(name)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to trigger scan')
    } finally {
      setScanning(null)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Watch Folders</h1>
      </div>

      <div className="bg-th-surface rounded-lg shadow p-4 text-sm text-th-text-muted">
        Watch folders automatically detect new media files and create source records + schedule
        analysis jobs (HDR detect + scene scan). <strong className="text-th-text">No encoding jobs
        are created automatically</strong> — after analysis completes you manually create jobs
        from the Sources page.
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {folders.length === 0 ? (
        <div className="bg-th-surface rounded-lg shadow p-8 text-center text-th-text-muted">
          <p className="text-lg mb-2">No watch folders configured</p>
          <p className="text-sm">Add <code className="bg-th-surface-muted px-1 rounded">watch_folders</code> to your <code className="bg-th-surface-muted px-1 rounded">config.yaml</code> to enable automatic file detection.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {folders.map(f => (
            <div key={f.name} className="bg-th-surface rounded-lg shadow overflow-hidden">
              <div className="flex items-center justify-between px-4 py-3 border-b border-th-border-subtle">
                <div className="flex items-center gap-3">
                  <span className={`w-2.5 h-2.5 rounded-full flex-shrink-0 ${f.enabled ? 'bg-green-500' : 'bg-gray-400'}`} />
                  <div>
                    <span className="font-semibold text-th-text">{f.name}</span>
                    {f.move_after_analysis && (
                      <span className="ml-2 text-xs text-th-text-muted">→ {f.move_after_analysis}</span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => handleScan(f.name)}
                    disabled={!f.enabled || scanning === f.name}
                    className="text-xs px-2 py-1 rounded border border-th-border text-th-text-secondary hover:bg-th-surface-muted disabled:opacity-40"
                  >
                    {scanning === f.name ? 'Scanning…' : 'Scan Now'}
                  </button>
                  <button
                    onClick={() => handleToggle(f.name, f.enabled)}
                    disabled={toggling === f.name}
                    className={`text-xs px-3 py-1 rounded font-medium disabled:opacity-50 ${
                      f.enabled
                        ? 'bg-red-100 text-red-700 hover:bg-red-200'
                        : 'bg-green-100 text-green-700 hover:bg-green-200'
                    }`}
                  >
                    {toggling === f.name ? '…' : f.enabled ? 'Disable' : 'Enable'}
                  </button>
                </div>
              </div>
              <div className="px-4 py-3 grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
                <div>
                  <span className="text-th-text-muted text-xs uppercase tracking-wide">Linux Path</span>
                  <p className="font-mono text-th-text text-xs mt-0.5">{f.path}</p>
                </div>
                <div>
                  <span className="text-th-text-muted text-xs uppercase tracking-wide">Windows Path (UNC)</span>
                  <p className="font-mono text-th-text text-xs mt-0.5">{f.windows_path || '—'}</p>
                </div>
                <div>
                  <span className="text-th-text-muted text-xs uppercase tracking-wide">File Patterns</span>
                  <p className="text-th-text text-xs mt-0.5">{f.file_patterns.join(', ')}</p>
                </div>
                <div>
                  <span className="text-th-text-muted text-xs uppercase tracking-wide">Poll Interval</span>
                  <p className="text-th-text text-xs mt-0.5">{f.poll_interval}</p>
                </div>
                <div>
                  <span className="text-th-text-muted text-xs uppercase tracking-wide">Auto Analyze</span>
                  <p className="text-th-text text-xs mt-0.5">{f.auto_analyze ? 'Yes' : 'No'}</p>
                </div>
                {f.last_scan && (
                  <div>
                    <span className="text-th-text-muted text-xs uppercase tracking-wide">Last Scan</span>
                    <p className="text-th-text text-xs mt-0.5">{new Date(f.last_scan).toLocaleString()}</p>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
