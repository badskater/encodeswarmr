import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { MediaServer } from '../../types'

const TYPE_LABELS: Record<string, string> = {
  plex: 'Plex',
  jellyfin: 'Jellyfin',
  emby: 'Emby',
}

export default function MediaServers() {
  const [servers, setServers] = useState<MediaServer[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [refreshing, setRefreshing] = useState<Record<string, boolean>>({})
  const [refreshResults, setRefreshResults] = useState<Record<string, string>>({})

  const load = useCallback(async () => {
    try {
      const result = await api.listMediaServers()
      setServers(result)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load media servers')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleRefresh = async (name: string) => {
    setRefreshing(prev => ({ ...prev, [name]: true }))
    setRefreshResults(prev => ({ ...prev, [name]: '' }))
    try {
      await api.refreshMediaServer(name)
      setRefreshResults(prev => ({ ...prev, [name]: 'Refresh triggered successfully' }))
    } catch (e: unknown) {
      setRefreshResults(prev => ({
        ...prev,
        [name]: e instanceof Error ? e.message : 'Refresh failed',
      }))
    } finally {
      setRefreshing(prev => ({ ...prev, [name]: false }))
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Media Servers</h1>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {servers.length === 0 ? (
        <div className="bg-th-surface rounded-lg shadow p-6 text-center text-th-text-muted text-sm">
          <p>No media servers configured.</p>
          <p className="mt-1 text-xs">
            Add a <code className="font-mono">media_servers</code> section to your controller config to enable integrations.
          </p>
        </div>
      ) : (
        <div className="bg-th-surface rounded-lg shadow overflow-hidden">
          <table className="min-w-full divide-y divide-th-border text-sm">
            <thead className="bg-th-surface-muted">
              <tr>
                {['Name', 'Type', 'Auto Refresh', 'Actions'].map(h => (
                  <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-th-border-subtle">
              {servers.map(srv => (
                <tr key={srv.name} className="hover:bg-th-surface-muted">
                  <td className="px-4 py-3 font-medium text-th-text">{srv.name}</td>
                  <td className="px-4 py-3 text-th-text-secondary">{TYPE_LABELS[srv.type] ?? srv.type}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
                      srv.auto_refresh
                        ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                        : 'bg-th-surface-muted text-th-text-muted'
                    }`}>
                      {srv.auto_refresh ? 'Enabled' : 'Disabled'}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-3">
                      <button
                        onClick={() => handleRefresh(srv.name)}
                        disabled={refreshing[srv.name]}
                        className="text-xs px-3 py-1.5 rounded font-medium bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50"
                      >
                        {refreshing[srv.name] ? 'Refreshing…' : 'Refresh Now'}
                      </button>
                      {refreshResults[srv.name] && (
                        <span className={`text-xs ${
                          refreshResults[srv.name].includes('success')
                            ? 'text-green-600'
                            : 'text-red-600'
                        }`}>
                          {refreshResults[srv.name]}
                        </span>
                      )}
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
