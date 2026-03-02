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
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const navigate = useNavigate()

  const load = useCallback(async () => {
    try {
      const s = await api.listSources()
      setSources(s)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])
  useAutoRefresh(load)

  if (loading) return <p className="text-gray-500">Loading…</p>

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold text-gray-900">Sources</h1>
      {error && <p className="text-red-600 text-sm">{error}</p>}
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200 text-sm">
          <thead className="bg-gray-50">
            <tr>
              {['Filename', 'Path', 'Size', 'Duration', 'VMAF', 'State', 'Created'].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {sources.map(s => (
              <tr
                key={s.id}
                onClick={() => navigate(`/sources/${s.id}`)}
                className="hover:bg-gray-50 cursor-pointer"
              >
                <td className="px-4 py-2 font-medium text-gray-900">{s.filename}</td>
                <td className="px-4 py-2 text-gray-500 max-w-xs truncate">{s.path}</td>
                <td className="px-4 py-2 text-gray-700 whitespace-nowrap">{fmtBytes(s.size_bytes)}</td>
                <td className="px-4 py-2 text-gray-700 whitespace-nowrap">{fmtDuration(s.duration_sec)}</td>
                <td className="px-4 py-2 text-gray-700">
                  {s.vmaf_score != null ? s.vmaf_score.toFixed(1) : '—'}
                </td>
                <td className="px-4 py-2"><StatusBadge status={s.state} /></td>
                <td className="px-4 py-2 text-gray-500 whitespace-nowrap">{fmtDate(s.created_at)}</td>
              </tr>
            ))}
            {sources.length === 0 && (
              <tr><td colSpan={7} className="px-4 py-4 text-center text-gray-400">No sources found</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
