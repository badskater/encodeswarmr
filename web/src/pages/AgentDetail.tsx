import { useState, useEffect, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import * as api from '../api/client'
import type { Agent, AgentHealth, Task } from '../types'
import StatusBadge from '../components/StatusBadge'
import AgentMetricsGraph from '../components/AgentMetricsGraph'
import { useAutoRefresh } from '../hooks/useAutoRefresh'

function fmtDuration(secs: number): string {
  if (secs <= 0) return '—'
  const d = Math.floor(secs / 86400)
  const h = Math.floor((secs % 86400) / 3600)
  const m = Math.floor((secs % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function fmtDate(s: string | null | undefined) {
  return s ? new Date(s).toLocaleString() : '—'
}

function fmtNum(n: number, decimals = 1) {
  return n.toFixed(decimals)
}

// Simple circular-gauge component using SVG.
function Gauge({ value, max = 100, label, color = '#3b82f6' }: {
  value: number
  max?: number
  label: string
  color?: string
}) {
  const r = 28
  const circ = 2 * Math.PI * r
  const pct = Math.min(value / max, 1)
  const dash = pct * circ
  return (
    <div className="flex flex-col items-center gap-1">
      <svg width="72" height="72" viewBox="0 0 72 72">
        <circle cx="36" cy="36" r={r} fill="none" stroke="currentColor" strokeOpacity={0.1} strokeWidth={8} />
        <circle
          cx="36" cy="36" r={r}
          fill="none"
          stroke={color}
          strokeWidth={8}
          strokeDasharray={`${dash} ${circ}`}
          strokeLinecap="round"
          transform="rotate(-90 36 36)"
        />
        <text x="36" y="40" textAnchor="middle" fontSize="12" fontWeight="bold" fill="currentColor">
          {fmtNum(value, 0)}%
        </text>
      </svg>
      <span className="text-xs text-th-text-muted">{label}</span>
    </div>
  )
}

export default function AgentDetail() {
  const { id } = useParams<{ id: string }>()
  const [agent, setAgent] = useState<Agent | null>(null)
  const [health, setHealth] = useState<AgentHealth | null>(null)
  const [recentTasks, setRecentTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [draining, setDraining] = useState(false)
  const [approving, setApproving] = useState(false)
  const [upgrading, setUpgrading] = useState(false)

  const load = useCallback(async () => {
    if (!id) return
    try {
      const [ag, h, tasks] = await Promise.all([
        api.getAgent(id),
        api.getAgentHealth(id),
        api.listAgentRecentTasks(id),
      ])
      setAgent(ag)
      setHealth(h)
      setRecentTasks(tasks ?? [])
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load agent')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { load() }, [load])
  useAutoRefresh(load)

  const handleDrain = async () => {
    if (!id) return
    setDraining(true)
    try { await api.drainAgent(id); load() } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to drain agent')
    } finally { setDraining(false) }
  }

  const handleApprove = async () => {
    if (!id) return
    setApproving(true)
    try { await api.approveAgent(id); load() } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to approve agent')
    } finally { setApproving(false) }
  }

  const handleUpgrade = async () => {
    if (!id) return
    setUpgrading(true)
    try { await api.requestAgentUpgrade(id); load() } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to request upgrade')
    } finally { setUpgrading(false) }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>
  if (!agent) return <p className="text-th-text-muted">Agent not found.</p>

  const gpuName = health?.gpu?.name?.trim() || `${agent.gpu_vendor} ${agent.gpu_model}`.trim() || '—'

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Link to="/agents" className="text-sm text-th-text-muted hover:underline">← Agents</Link>
          <h1 className="text-2xl font-bold text-th-text">{agent.name}</h1>
          <StatusBadge status={agent.status} />
          {agent.tags.length > 0 && (
            <div className="flex gap-1">
              {agent.tags.map(t => (
                <span key={t} className="text-xs px-1.5 py-0.5 rounded bg-th-surface-muted text-th-text-secondary border border-th-border-subtle">{t}</span>
              ))}
            </div>
          )}
        </div>
        <div className="flex gap-2">
          {(agent.status === 'idle' || agent.status === 'busy') && (
            <button
              onClick={handleDrain}
              disabled={draining}
              className="text-sm px-3 py-1.5 rounded disabled:opacity-50"
              style={{ backgroundColor: 'var(--th-badge-draining-bg)', color: 'var(--th-badge-draining-text)' }}
            >
              {draining ? 'Draining…' : 'Drain'}
            </button>
          )}
          {agent.status === 'pending_approval' && (
            <button
              onClick={handleApprove}
              disabled={approving}
              className="text-sm px-3 py-1.5 rounded disabled:opacity-50"
              style={{ backgroundColor: 'var(--th-badge-success-bg)', color: 'var(--th-badge-success-text)' }}
            >
              {approving ? 'Approving…' : 'Approve'}
            </button>
          )}
          <button
            onClick={handleUpgrade}
            disabled={upgrading || agent.upgrade_requested}
            className="text-sm px-3 py-1.5 rounded border border-th-border text-th-text hover:bg-th-surface-muted disabled:opacity-50"
          >
            {agent.upgrade_requested ? 'Upgrade Queued' : upgrading ? 'Requesting…' : 'Request Upgrade'}
          </button>
          {agent.vnc_port > 0 && (
            <a
              href={`/novnc/${agent.id}`}
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm px-3 py-1.5 rounded no-underline"
              style={{ backgroundColor: 'var(--th-badge-running-bg)', color: 'var(--th-badge-running-text)' }}
            >
              Remote Desktop
            </a>
          )}
        </div>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {/* Info row */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {[
          { label: 'Hostname', value: agent.hostname },
          { label: 'IP Address', value: agent.ip_address },
          { label: 'OS', value: agent.os_version || '—' },
          { label: 'Agent Version', value: agent.agent_version || '—' },
          { label: 'CPU Cores', value: String(agent.cpu_count) },
          { label: 'RAM', value: `${(agent.ram_mib / 1024).toFixed(0)} GB` },
          { label: 'Uptime', value: fmtDuration(health?.uptime_seconds ?? 0) },
          { label: 'Last Heartbeat', value: fmtDate(agent.last_heartbeat) },
        ].map(({ label, value }) => (
          <div key={label} className="bg-th-surface rounded-lg shadow p-3">
            <p className="text-xs text-th-text-muted mb-0.5">{label}</p>
            <p className="text-sm font-medium text-th-text">{value}</p>
          </div>
        ))}
      </div>

      {/* CPU / Memory / GPU gauges */}
      <div className="bg-th-surface rounded-lg shadow p-5">
        <h2 className="text-sm font-semibold text-th-text mb-4">Resource Utilisation</h2>
        <div className="flex flex-wrap gap-8 justify-around">
          <Gauge value={health?.cpu_usage_pct ?? 0} label="CPU" color="#3b82f6" />
          <Gauge value={health?.memory_usage_pct ?? 0} label="Memory" color="#f59e0b" />
          {agent.gpu_enabled && (
            <Gauge value={health?.gpu?.utilization_pct ?? 0} label="GPU" color="#22c55e" />
          )}
        </div>
      </div>

      {/* GPU info (if enabled) */}
      {agent.gpu_enabled && (
        <div className="bg-th-surface rounded-lg shadow p-5 space-y-2">
          <h2 className="text-sm font-semibold text-th-text">GPU</h2>
          <p className="text-sm text-th-text-secondary">{gpuName}</p>
          <div className="flex gap-4 text-xs text-th-text-muted">
            {agent.nvenc && <span className="rounded bg-th-surface-muted px-1.5 py-0.5 border border-th-border-subtle">NVENC</span>}
            {agent.qsv && <span className="rounded bg-th-surface-muted px-1.5 py-0.5 border border-th-border-subtle">QSV</span>}
            {agent.amf && <span className="rounded bg-th-surface-muted px-1.5 py-0.5 border border-th-border-subtle">AMF</span>}
          </div>
        </div>
      )}

      {/* Encoding stats */}
      <div className="bg-th-surface rounded-lg shadow p-5">
        <h2 className="text-sm font-semibold text-th-text mb-4">Encoding Statistics</h2>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          {[
            { label: 'Tasks Completed', value: String(health?.encoding_stats?.total_tasks_completed ?? 0) },
            { label: 'Avg FPS', value: fmtNum(health?.encoding_stats?.avg_fps ?? 0) },
            { label: 'Total Frames', value: (health?.encoding_stats?.total_frames_encoded ?? 0).toLocaleString() },
            { label: 'Error Rate', value: `${fmtNum(health?.encoding_stats?.error_rate_pct ?? 0)}%` },
          ].map(({ label, value }) => (
            <div key={label}>
              <p className="text-xs text-th-text-muted mb-0.5">{label}</p>
              <p className="text-lg font-semibold text-th-text">{value}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Historical metrics graph */}
      <div className="bg-th-surface rounded-lg shadow p-5">
        <h2 className="text-sm font-semibold text-th-text mb-3">Historical Metrics — last hour</h2>
        <AgentMetricsGraph agentId={agent.id} />
      </div>

      {/* Recent tasks */}
      <div className="bg-th-surface rounded-lg shadow overflow-hidden">
        <div className="px-5 py-3 border-b border-th-border-subtle">
          <h2 className="text-sm font-semibold text-th-text">Recent Tasks</h2>
        </div>
        <table className="min-w-full divide-y divide-th-border text-sm">
          <thead className="bg-th-surface-muted">
            <tr>
              {['ID', 'Job', 'Chunk', 'Status', 'FPS', 'Frames', 'Duration', 'Completed'].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-th-border-subtle">
            {recentTasks.map(t => (
              <tr key={t.id} className="hover:bg-th-surface-muted">
                <td className="px-4 py-2">
                  <Link to={`/tasks/${t.id}`} className="text-blue-500 hover:underline font-mono text-xs">
                    {t.id.slice(0, 8)}…
                  </Link>
                </td>
                <td className="px-4 py-2">
                  <Link to={`/jobs/${t.job_id}`} className="text-blue-500 hover:underline font-mono text-xs">
                    {t.job_id.slice(0, 8)}…
                  </Link>
                </td>
                <td className="px-4 py-2 text-th-text-secondary">{t.chunk_index}</td>
                <td className="px-4 py-2"><StatusBadge status={t.status} /></td>
                <td className="px-4 py-2 text-th-text-secondary">{t.avg_fps != null ? fmtNum(t.avg_fps) : '—'}</td>
                <td className="px-4 py-2 text-th-text-secondary">{t.frames_encoded != null ? t.frames_encoded.toLocaleString() : '—'}</td>
                <td className="px-4 py-2 text-th-text-secondary">{t.duration_sec != null ? fmtDuration(t.duration_sec) : '—'}</td>
                <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(t.completed_at)}</td>
              </tr>
            ))}
            {recentTasks.length === 0 && (
              <tr>
                <td colSpan={8} className="px-4 py-4 text-center text-th-text-subtle">No tasks found for this agent</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
