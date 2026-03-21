import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import * as api from '../api/client'
import type { Job, Agent, ThroughputPoint, QueueSummary, ActivityEvent } from '../types'
import type { AgentMetric } from '../api/client'
import StatusBadge from '../components/StatusBadge'
import ProgressBar from '../components/ProgressBar'
import { useAutoRefresh } from '../hooks/useAutoRefresh'

function basename(p: string) {
  return p.split(/[\\/]/).pop() ?? p
}

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

function fmtDuration(sec: number | null) {
  if (sec == null || sec <= 0) return null
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (h > 0) return `~${h}h ${m}m`
  return `~${m}m`
}

// ── Throughput chart (jobs completed per hour over last 24 h) ──────────────────

const CHART_W = 480
const CHART_H = 100
const CHART_PAD = { top: 8, right: 8, bottom: 20, left: 28 }

function ThroughputChart({ data }: { data: ThroughputPoint[] }) {
  if (data.length === 0) {
    return <p className="text-xs text-th-text-subtle py-4 text-center">No completed jobs in the last 24 h</p>
  }

  const innerW = CHART_W - CHART_PAD.left - CHART_PAD.right
  const innerH = CHART_H - CHART_PAD.top - CHART_PAD.bottom
  const maxVal = Math.max(...data.map(d => d.completed), 1)
  const barW = Math.max(2, innerW / data.length - 2)

  return (
    <svg
      viewBox={`0 0 ${CHART_W} ${CHART_H}`}
      width={CHART_W}
      height={CHART_H}
      style={{ display: 'block', maxWidth: '100%' }}
      aria-label="Encoding throughput over last 24 hours"
    >
      {/* Y-axis grid */}
      {[0, Math.round(maxVal / 2), maxVal].map(v => {
        const y = CHART_PAD.top + innerH - (v / maxVal) * innerH
        return (
          <g key={v}>
            <line
              x1={CHART_PAD.left} y1={y}
              x2={CHART_PAD.left + innerW} y2={y}
              stroke="currentColor" strokeOpacity={0.1} strokeWidth={1}
            />
            <text x={CHART_PAD.left - 4} y={y + 3} fontSize={8} textAnchor="end" fill="currentColor" opacity={0.4}>
              {v}
            </text>
          </g>
        )
      })}

      {/* Bars */}
      {data.map((d, i) => {
        const x = CHART_PAD.left + (i / data.length) * innerW
        const barH = (d.completed / maxVal) * innerH
        const y = CHART_PAD.top + innerH - barH
        return (
          <rect
            key={d.hour}
            x={x + 1}
            y={y}
            width={barW}
            height={Math.max(barH, 0)}
            fill="var(--th-badge-running-text, #3b82f6)"
            fillOpacity={0.7}
            rx={1}
          >
            <title>{new Date(d.hour).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })} — {d.completed} jobs</title>
          </rect>
        )
      })}

      {/* X-axis baseline */}
      <line
        x1={CHART_PAD.left} y1={CHART_PAD.top + innerH}
        x2={CHART_PAD.left + innerW} y2={CHART_PAD.top + innerH}
        stroke="currentColor" strokeOpacity={0.15} strokeWidth={1}
      />

      {/* X-axis label: first and last hour */}
      {data.length > 0 && (
        <>
          <text x={CHART_PAD.left} y={CHART_H - 4} fontSize={8} fill="currentColor" opacity={0.4}>
            {new Date(data[0].hour).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </text>
          <text x={CHART_PAD.left + innerW} y={CHART_H - 4} fontSize={8} textAnchor="end" fill="currentColor" opacity={0.4}>
            {new Date(data[data.length - 1].hour).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </text>
        </>
      )}
    </svg>
  )
}

// ── Agent utilisation mini-bars ───────────────────────────────────────────────

function AgentUtilBar({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="flex items-center gap-1.5 text-xs">
      <span className="w-8 text-th-text-muted shrink-0">{label}</span>
      <div className="flex-1 bg-th-surface rounded-full h-1.5 overflow-hidden">
        <div
          className="h-full rounded-full transition-all"
          style={{ width: `${Math.min(value, 100)}%`, backgroundColor: color }}
        />
      </div>
      <span className="w-8 text-right text-th-text-subtle">{value.toFixed(0)}%</span>
    </div>
  )
}

function AgentUtilSummary({ agents }: { agents: Agent[] }) {
  const [latestMetrics, setLatestMetrics] = useState<Record<string, AgentMetric>>({})

  useEffect(() => {
    const online = agents.filter(a => a.status !== 'offline')
    if (online.length === 0) return

    Promise.all(
      online.map(a =>
        api.listAgentMetrics(a.id, '5m')
          .then(metrics => ({ id: a.id, latest: metrics[metrics.length - 1] ?? null }))
          .catch(() => ({ id: a.id, latest: null }))
      )
    ).then(results => {
      const map: Record<string, AgentMetric> = {}
      results.forEach(r => { if (r.latest) map[r.id] = r.latest })
      setLatestMetrics(map)
    })
  }, [agents])

  const online = agents.filter(a => a.status !== 'offline')

  if (online.length === 0) {
    return <p className="text-xs text-th-text-subtle">No online agents</p>
  }

  return (
    <div className="space-y-3">
      {online.map(a => {
        const m = latestMetrics[a.id]
        return (
          <div key={a.id} className="space-y-1">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-th-text truncate max-w-[120px]">{a.name}</span>
              <StatusBadge status={a.status} />
            </div>
            {m ? (
              <div className="space-y-0.5">
                <AgentUtilBar label="CPU" value={m.cpu_pct} color="var(--th-badge-running-text, #3b82f6)" />
                {a.gpu_enabled && (
                  <AgentUtilBar label="GPU" value={m.gpu_pct} color="var(--th-badge-success-text, #22c55e)" />
                )}
                <AgentUtilBar label="Mem" value={m.mem_pct} color="var(--th-badge-draining-text, #f59e0b)" />
              </div>
            ) : (
              <p className="text-xs text-th-text-subtle">No recent metrics</p>
            )}
          </div>
        )
      })}
    </div>
  )
}

// ── Queue depth indicator ─────────────────────────────────────────────────────

function QueueDepth({ queue }: { queue: QueueSummary | null }) {
  if (!queue) return <p className="text-xs text-th-text-subtle">Loading…</p>
  const eta = fmtDuration(queue.estimated_completion_sec)
  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-2 gap-3">
        {[
          { label: 'Pending', value: queue.pending, color: 'var(--th-badge-warning-text, #f59e0b)' },
          { label: 'Running', value: queue.running, color: 'var(--th-badge-running-text, #3b82f6)' },
        ].map(item => (
          <div key={item.label} className="bg-th-surface-muted rounded p-3 text-center">
            <p className="text-xs text-th-text-muted">{item.label}</p>
            <p className="text-2xl font-bold mt-0.5" style={{ color: item.color }}>{item.value}</p>
          </div>
        ))}
      </div>
      {eta && (
        <p className="text-xs text-th-text-muted text-center">
          Est. completion: <span className="text-th-text font-medium">{eta}</span>
        </p>
      )}
      {!eta && queue.pending === 0 && queue.running === 0 && (
        <p className="text-xs text-th-text-subtle text-center">Queue is empty</p>
      )}
    </div>
  )
}

// ── Recent activity feed ──────────────────────────────────────────────────────

function ActivityFeed({ events }: { events: ActivityEvent[] }) {
  if (events.length === 0) {
    return <p className="text-xs text-th-text-subtle text-center py-2">No recent activity</p>
  }
  return (
    <ul className="space-y-2">
      {events.map((e, i) => (
        <li key={`${e.job_id}-${i}`} className="flex items-start gap-2 text-xs">
          <StatusBadge status={e.status} />
          <div className="flex-1 min-w-0">
            <Link to={`/jobs/${e.job_id}`} className="text-blue-600 hover:underline font-mono">
              {e.job_id.slice(0, 8)}
            </Link>
            {' '}
            <span className="text-th-text-secondary truncate">{basename(e.source_path)}</span>
          </div>
          <span className="text-th-text-subtle whitespace-nowrap shrink-0">
            {new Date(e.changed_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </span>
        </li>
      ))}
    </ul>
  )
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

export default function Dashboard() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [agents, setAgents] = useState<Agent[]>([])
  const [throughput, setThroughput] = useState<ThroughputPoint[]>([])
  const [queue, setQueue] = useState<QueueSummary | null>(null)
  const [activity, setActivity] = useState<ActivityEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    try {
      const [j, a, tp, q, act] = await Promise.all([
        api.listJobs(),
        api.listAgents(),
        api.getThroughput(24).catch(() => [] as ThroughputPoint[]),
        api.getQueueSummary().catch(() => null),
        api.getRecentActivity(10).catch(() => [] as ActivityEvent[]),
      ])
      setJobs(j)
      setAgents(a)
      setThroughput(tp)
      setQueue(q)
      setActivity(act)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])
  useAutoRefresh(load)

  const running = jobs.filter(j => j.status === 'running').length
  const idleAgents = agents.filter(a => a.status === 'idle').length
  const offlineAgents = agents.filter(a => a.status === 'offline').length
  const recent = [...jobs].sort((a, b) => b.created_at.localeCompare(a.created_at)).slice(0, 10)

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-th-text">Dashboard</h1>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {/* Summary stat cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        {[
          { label: 'Total Jobs', value: jobs.length },
          { label: 'Running Jobs', value: running },
          { label: 'Idle Agents', value: idleAgents },
          { label: 'Offline Agents', value: offlineAgents },
        ].map(card => (
          <div key={card.label} className="bg-th-surface rounded-lg shadow p-4">
            <p className="text-sm text-th-text-muted">{card.label}</p>
            <p className="text-3xl font-bold text-th-text mt-1">{card.value}</p>
          </div>
        ))}
      </div>

      {/* Second row: throughput chart + queue + agent util */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">

        {/* Encoding throughput chart */}
        <div className="lg:col-span-2 bg-th-surface rounded-lg shadow">
          <div className="px-4 py-3 border-b border-th-border">
            <h2 className="text-sm font-semibold text-th-text-secondary">Encoding Throughput (last 24 h)</h2>
          </div>
          <div className="px-4 py-3 overflow-x-auto">
            <ThroughputChart data={throughput} />
          </div>
        </div>

        {/* Queue depth */}
        <div className="bg-th-surface rounded-lg shadow">
          <div className="px-4 py-3 border-b border-th-border">
            <h2 className="text-sm font-semibold text-th-text-secondary">Queue Depth</h2>
          </div>
          <div className="px-4 py-3">
            <QueueDepth queue={queue} />
          </div>
        </div>
      </div>

      {/* Third row: agent utilisation + activity feed */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">

        {/* Agent utilisation */}
        <div className="bg-th-surface rounded-lg shadow">
          <div className="px-4 py-3 border-b border-th-border">
            <h2 className="text-sm font-semibold text-th-text-secondary">Agent Utilisation</h2>
          </div>
          <div className="px-4 py-3 space-y-3 max-h-72 overflow-y-auto">
            <AgentUtilSummary agents={agents} />
          </div>
        </div>

        {/* Recent activity feed */}
        <div className="bg-th-surface rounded-lg shadow">
          <div className="px-4 py-3 border-b border-th-border">
            <h2 className="text-sm font-semibold text-th-text-secondary">Recent Activity</h2>
          </div>
          <div className="px-4 py-3 max-h-72 overflow-y-auto">
            <ActivityFeed events={activity} />
          </div>
        </div>
      </div>

      {/* Recent jobs table */}
      <div className="bg-th-surface rounded-lg shadow">
        <div className="px-4 py-3 border-b border-th-border">
          <h2 className="text-sm font-semibold text-th-text-secondary">Recent Jobs</h2>
        </div>

        {/* Desktop table */}
        <div className="hidden sm:block">
          <table className="min-w-full divide-y divide-th-border text-sm">
            <thead className="bg-th-surface-muted">
              <tr>
                {['ID', 'Source', 'Status', 'Progress', 'Created'].map(h => (
                  <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-th-border-subtle">
              {recent.map(j => (
                <tr key={j.id} className="hover:bg-th-surface-muted">
                  <td className="px-4 py-2 font-mono">
                    <Link to={`/jobs/${j.id}`} className="text-blue-600 hover:underline">{j.id.slice(0, 8)}</Link>
                  </td>
                  <td className="px-4 py-2 max-w-xs truncate text-th-text-secondary">{basename(j.source_path)}</td>
                  <td className="px-4 py-2"><StatusBadge status={j.status} /></td>
                  <td className="px-4 py-2 w-32">
                    <ProgressBar value={j.tasks_completed} max={j.tasks_total} />
                    <span className="text-xs text-th-text-subtle">{j.tasks_completed}/{j.tasks_total}</span>
                  </td>
                  <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(j.created_at)}</td>
                </tr>
              ))}
              {recent.length === 0 && (
                <tr><td colSpan={5} className="px-4 py-4 text-center text-th-text-subtle">No jobs yet</td></tr>
              )}
            </tbody>
          </table>
        </div>

        {/* Mobile card list */}
        <div className="sm:hidden divide-y divide-th-border-subtle">
          {recent.map(j => (
            <Link key={j.id} to={`/jobs/${j.id}`} className="block px-4 py-3 hover:bg-th-surface-muted">
              <div className="flex items-center justify-between mb-1">
                <span className="font-mono text-xs text-blue-600">{j.id.slice(0, 8)}</span>
                <StatusBadge status={j.status} />
              </div>
              <p className="text-sm text-th-text-secondary truncate">{basename(j.source_path)}</p>
              <div className="mt-1.5">
                <ProgressBar value={j.tasks_completed} max={j.tasks_total} />
                <div className="flex justify-between mt-0.5 text-xs text-th-text-subtle">
                  <span>{j.tasks_completed}/{j.tasks_total} tasks</span>
                  <span>{fmtDate(j.created_at)}</span>
                </div>
              </div>
            </Link>
          ))}
          {recent.length === 0 && (
            <p className="px-4 py-4 text-center text-th-text-subtle text-sm">No jobs yet</p>
          )}
        </div>
      </div>
    </div>
  )
}
