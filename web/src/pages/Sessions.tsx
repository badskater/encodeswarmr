import { useState, useEffect, useCallback } from 'react'
import * as api from '../api/client'

export default function Sessions() {
  const [sessions, setSessions] = useState<api.UserSession[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [revoking, setRevoking] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const data = await api.listSessions()
      setSessions(data)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load sessions')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleRevoke = async (token: string) => {
    if (!confirm('Revoke this session? You will be logged out if it is your current session.')) return
    setRevoking(token)
    try {
      await api.deleteSession(token)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to revoke session')
    } finally {
      setRevoking(null)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-6 max-w-4xl">
      <div>
        <h1 className="text-2xl font-bold text-th-text">Active Sessions</h1>
        <p className="text-sm text-th-text-muted mt-0.5">
          Manage your active login sessions. Revoking a session will log out that device.
        </p>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {sessions.length === 0 ? (
        <p className="text-sm text-th-text-muted">No active sessions found.</p>
      ) : (
        <div className="bg-th-surface rounded-lg shadow overflow-hidden">
          <table className="min-w-full text-sm">
            <thead className="bg-th-surface-muted">
              <tr className="text-left text-th-text-muted border-b border-th-border">
                <th className="px-4 py-2 font-medium">Session</th>
                <th className="px-4 py-2 font-medium">Created</th>
                <th className="px-4 py-2 font-medium">Expires</th>
                <th className="px-4 py-2 font-medium"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-th-border">
              {sessions.map(s => (
                <tr key={s.token} className="hover:bg-th-surface-muted">
                  <td className="px-4 py-2 font-mono text-xs text-th-text-muted">
                    {s.token.slice(0, 12)}…
                  </td>
                  <td className="px-4 py-2 text-th-text-muted text-xs">
                    {new Date(s.created_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-2 text-th-text-muted text-xs">
                    {new Date(s.expires_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-2 text-right">
                    <button
                      onClick={() => handleRevoke(s.token)}
                      disabled={revoking === s.token}
                      className="text-sm text-red-500 hover:underline disabled:opacity-50"
                    >
                      {revoking === s.token ? 'Revoking…' : 'Revoke'}
                    </button>
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
