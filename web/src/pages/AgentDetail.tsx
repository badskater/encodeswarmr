import { useState, useEffect, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import * as api from '../api/client'
import type { Agent } from '../types'

const UPDATE_CHANNELS = ['stable', 'beta', 'nightly'] as const

const channelBadgeClass = (channel: string) => {
  switch (channel) {
    case 'beta': return 'bg-yellow-100 text-yellow-800'
    case 'nightly': return 'bg-purple-100 text-purple-800'
    default: return 'bg-green-100 text-green-800'
  }
}

export default function AgentDetail() {
  const { id } = useParams<{ id: string }>()
  const [health, setHealth] = useState<api.AgentHealthResponse | null>(null)
  const [recentTasks, setRecentTasks] = useState<api.RecentTask[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [channelUpdating, setChannelUpdating] = useState(false)
  const [channelMsg, setChannelMsg] = useState('')

  const load = useCallback(async () => {
    if (!id) return
    try {
      const [h, tasks] = await Promise.all([
        api.getAgentHealth(id),
        api.listAgentRecentTasks(id),
      ])
      setHealth(h)
      setRecentTasks(tasks)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load agent')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { load() }, [load])

  const handleChannelChange = async (channel: string) => {
    if (!id) return
    setChannelUpdating(true)
    setChannelMsg('')
    try {
      await api.updateAgentChannel(id, channel)
      setChannelMsg(`Update channel set to "${channel}".`)
      load()
    } catch (e: unknown) {
      setChannelMsg(e instanceof Error ? e.message : 'Failed to update channel')
    } finally {
      setChannelUpdating(false)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>
  if (error) return <p className="text-red-600">{error}</p>
  if (!health) return null

  const { agent, encoding_stats: stats } = health
  const agentWithChannel = agent as Agent & { update_channel?: string }

  return (
    <div className="space-y-6 max-w-4xl">
      <div className="flex items-center gap-3">
        <Link to="/agents" className="text-blue-600 hover:underline text-sm">← Agents</Link>
        <h1 className="text-2xl font-bold text-th-text">{agent.name}</h1>
        <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
          agent.status === 'idle' ? 'bg-green-100 text-green-800' :
          agent.status === 'running' ? 'bg-blue-100 text-blue-800' :
          agent.status === 'offline' ? 'bg-gray-100 text-gray-600' :
          'bg-yellow-100 text-yellow-800'
        }`}>{agent.status}</span>
      </div>

      {/* Agent info */}
      <section className="bg-th-surface rounded-lg shadow p-5 grid grid-cols-2 md:grid-cols-4 gap-4">
        <InfoItem label="Hostname" value={agent.hostname} />
        <InfoItem label="IP Address" value={agent.ip_address} />
        <InfoItem label="Version" value={agent.agent_version} />
        <InfoItem label="OS" value={agent.os_version} />
        <InfoItem label="CPUs" value={String(agent.cpu_count)} />
        <InfoItem label="RAM" value={`${Math.round(agent.ram_mib / 1024)} GiB`} />
        <InfoItem label="GPU" value={agent.gpu_model || '—'} />
        <InfoItem label="Tags" value={agent.tags.length ? agent.tags.join(', ') : '—'} />
      </section>

      {/* Encoding stats */}
      <section className="bg-th-surface rounded-lg shadow p-5">
        <h2 className="text-sm font-semibold text-th-text mb-4">Encoding Statistics</h2>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
          <StatCard label="Total Tasks" value={String(stats.total_tasks)} />
          <StatCard label="Completed" value={String(stats.completed_tasks)} color="text-green-600" />
          <StatCard label="Failed" value={String(stats.failed_tasks)} color="text-red-500" />
          <StatCard label="Avg FPS" value={stats.avg_fps > 0 ? stats.avg_fps.toFixed(1) : '—'} />
          <StatCard label="Total Frames" value={stats.total_frames > 0 ? stats.total_frames.toLocaleString() : '—'} />
          <StatCard
            label="Success Rate"
            value={stats.total_tasks > 0
              ? `${((stats.completed_tasks / stats.total_tasks) * 100).toFixed(1)}%`
              : '—'}
          />
        </div>
      </section>

      {/* Update channel */}
      <section className="bg-th-surface rounded-lg shadow p-5 space-y-3">
        <h2 className="text-sm font-semibold text-th-text">Update Channel</h2>
        <div className="flex items-center gap-3 flex-wrap">
          <span className="text-sm text-th-text-muted">Current:</span>
          <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${channelBadgeClass(agentWithChannel.update_channel || 'stable')}`}>
            {agentWithChannel.update_channel || 'stable'}
          </span>
        </div>
        <div className="flex gap-2 flex-wrap">
          {UPDATE_CHANNELS.map(ch => (
            <button
              key={ch}
              onClick={() => handleChannelChange(ch)}
              disabled={channelUpdating || (agentWithChannel.update_channel || 'stable') === ch}
              className="rounded border border-th-border px-3 py-1.5 text-sm text-th-text hover:bg-th-surface-muted disabled:opacity-50 disabled:cursor-default"
            >
              {ch}
            </button>
          ))}
        </div>
        {channelMsg && (
          <p className={`text-xs ${channelMsg.includes('set to') ? 'text-green-600' : 'text-red-500'}`}>
            {channelMsg}
          </p>
        )}
      </section>

      {/* Recent tasks */}
      <section className="bg-th-surface rounded-lg shadow p-5">
        <h2 className="text-sm font-semibold text-th-text mb-3">Recent Tasks</h2>
        {recentTasks.length === 0 ? (
          <p className="text-sm text-th-text-muted">No tasks found.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="text-left text-th-text-muted border-b border-th-border">
                  <th className="pb-2 pr-4 font-medium">Task</th>
                  <th className="pb-2 pr-4 font-medium">Job</th>
                  <th className="pb-2 pr-4 font-medium">Status</th>
                  <th className="pb-2 pr-4 font-medium">FPS</th>
                  <th className="pb-2 font-medium">Updated</th>
                </tr>
              </thead>
              <tbody>
                {recentTasks.map(t => (
                  <tr key={t.id} className="border-b border-th-border/50">
                    <td className="py-1.5 pr-4">
                      <Link to={`/tasks/${t.id}`} className="text-blue-600 hover:underline font-mono text-xs">
                        {t.id.slice(0, 8)}…
                      </Link>
                    </td>
                    <td className="py-1.5 pr-4">
                      <Link to={`/jobs/${t.job_id}`} className="text-blue-600 hover:underline font-mono text-xs">
                        {t.job_id.slice(0, 8)}…
                      </Link>
                    </td>
                    <td className="py-1.5 pr-4">
                      <span className={`text-xs ${
                        t.status === 'completed' ? 'text-green-600' :
                        t.status === 'failed' ? 'text-red-500' :
                        t.status === 'running' ? 'text-blue-600' :
                        'text-th-text-muted'
                      }`}>{t.status}</span>
                    </td>
                    <td className="py-1.5 pr-4 text-th-text-muted">{t.avg_fps ? t.avg_fps.toFixed(1) : '—'}</td>
                    <td className="py-1.5 text-th-text-muted text-xs">{new Date(t.updated_at).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  )
}

function InfoItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs text-th-text-muted">{label}</p>
      <p className="text-sm text-th-text font-medium truncate">{value}</p>
    </div>
  )
}

function StatCard({ label, value, color = 'text-th-text' }: { label: string; value: string; color?: string }) {
  return (
    <div className="rounded border border-th-border p-3">
      <p className="text-xs text-th-text-muted">{label}</p>
      <p className={`text-xl font-bold mt-0.5 ${color}`}>{value}</p>
    </div>
  )
}
