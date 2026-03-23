import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { ActiveSession } from '../../types'

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

export default function Sessions() {
  const [sessions, setSessions] = useState<ActiveSession[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [terminating, setTerminating] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const s = await api.listSessions()
      setSessions(s)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load sessions')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleTerminate = async (id: string) => {
    if (!confirm('Terminate this session? The user will be logged out.')) return
    setTerminating(id)
    try {
      await api.terminateSession(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to terminate session')
    } finally {
      setTerminating(null)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Active Sessions</h1>
        <button
          onClick={load}
          className="text-sm px-3 py-1.5 rounded border border-th-input-border text-th-text-muted hover:bg-th-surface-muted"
        >
          Refresh
        </button>
      </div>
      {error && <p className="text-red-600 text-sm">{error}</p>}

      <div className="bg-th-surface rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-th-border text-sm">
          <thead className="bg-th-surface-muted">
            <tr>
              {['Session ID', 'User', 'Created', 'Expires', ''].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-th-border-subtle">
            {sessions.map(s => (
              <tr key={s.id} className="hover:bg-th-surface-muted">
                <td className="px-4 py-2 font-mono text-xs text-th-text-muted">{s.id}</td>
                <td className="px-4 py-2 font-medium text-th-text">{s.username}</td>
                <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(s.created_at)}</td>
                <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(s.expires_at)}</td>
                <td className="px-4 py-2">
                  <button
                    onClick={() => handleTerminate(s.id)}
                    disabled={terminating === s.id}
                    className="text-xs text-red-600 hover:underline disabled:opacity-50"
                  >
                    {terminating === s.id ? 'Terminating…' : 'Terminate'}
                  </button>
                </td>
              </tr>
            ))}
            {sessions.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-4 text-center text-th-text-subtle">No active sessions</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
