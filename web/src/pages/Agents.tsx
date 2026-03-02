import { useState, useEffect, useCallback } from 'react'
import * as api from '../api/client'
import type { Agent } from '../types'
import StatusBadge from '../components/StatusBadge'
import { useAutoRefresh } from '../hooks/useAutoRefresh'

function fmtBytes(n: number) {
  if (n >= 1024) return (n / 1024).toFixed(0) + ' GB'
  return n + ' MB'
}

function fmtDate(s: string | null) {
  return s ? new Date(s).toLocaleString() : '—'
}

export default function Agents() {
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [draining, setDraining] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const a = await api.listAgents()
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

  const handleDrain = async (id: string) => {
    setDraining(id)
    try {
      await api.drainAgent(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to drain agent')
    } finally {
      setDraining(null)
    }
  }

  if (loading) return <p className="text-gray-500">Loading…</p>

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold text-gray-900">Agents</h1>
      {error && <p className="text-red-600 text-sm">{error}</p>}
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200 text-sm">
          <thead className="bg-gray-50">
            <tr>
              {['Name', 'Hostname', 'IP', 'Status', 'CPU', 'RAM', 'GPU', 'Last Heartbeat', 'Tags', ''].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {agents.map(a => (
              <tr key={a.id} className="hover:bg-gray-50">
                <td className="px-4 py-2 font-medium text-gray-900">{a.name}</td>
                <td className="px-4 py-2 text-gray-700">{a.hostname}</td>
                <td className="px-4 py-2 text-gray-700">{a.ip_address}</td>
                <td className="px-4 py-2"><StatusBadge status={a.status} /></td>
                <td className="px-4 py-2 text-gray-700">{a.cpu_count} cores</td>
                <td className="px-4 py-2 text-gray-700">{fmtBytes(a.ram_mib)}</td>
                <td className="px-4 py-2 text-gray-700">
                  {a.gpu_enabled ? (
                    <span title={[a.nvenc && 'NVENC', a.qsv && 'QSV', a.amf && 'AMF'].filter(Boolean).join(', ')}>
                      {a.gpu_vendor} {a.gpu_model}
                    </span>
                  ) : '—'}
                </td>
                <td className="px-4 py-2 text-gray-500 whitespace-nowrap">{fmtDate(a.last_heartbeat)}</td>
                <td className="px-4 py-2 text-gray-500">
                  {a.tags.length > 0 ? a.tags.join(', ') : '—'}
                </td>
                <td className="px-4 py-2">
                  {(a.status === 'idle' || a.status === 'running') && (
                    <button
                      onClick={() => handleDrain(a.id)}
                      disabled={draining === a.id}
                      className="text-xs bg-orange-100 text-orange-800 px-2 py-1 rounded hover:bg-orange-200 disabled:opacity-50"
                    >
                      {draining === a.id ? 'Draining…' : 'Drain'}
                    </button>
                  )}
                </td>
              </tr>
            ))}
            {agents.length === 0 && (
              <tr><td colSpan={10} className="px-4 py-4 text-center text-gray-400">No agents registered</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
