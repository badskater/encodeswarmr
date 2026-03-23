import { Fragment, useState, useEffect, useCallback } from 'react'
import * as api from '../api/client'
import type { Agent } from '../types'
import StatusBadge from '../components/StatusBadge'
import AgentMetricsGraph from '../components/AgentMetricsGraph'
import { useAutoRefresh } from '../hooks/useAutoRefresh'

const CHANNEL_COLOURS: Record<string, { bg: string; text: string }> = {
  stable:  { bg: 'var(--th-badge-success-bg)',  text: 'var(--th-badge-success-text)' },
  beta:    { bg: 'var(--th-badge-running-bg)',   text: 'var(--th-badge-running-text)' },
  nightly: { bg: 'var(--th-badge-draining-bg)',  text: 'var(--th-badge-draining-text)' },
}

function ChannelBadge({ channel }: { channel: string }) {
  const c = CHANNEL_COLOURS[channel] ?? CHANNEL_COLOURS.stable
  return (
    <span
      className="text-xs px-1.5 py-0.5 rounded font-medium"
      style={{ backgroundColor: c.bg, color: c.text }}
    >
      {channel}
    </span>
  )
}

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
  const [approving, setApproving] = useState<string | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [bulkApproving, setBulkApproving] = useState(false)
  const [expandedMetrics, setExpandedMetrics] = useState<string | null>(null)
  const [changingChannel, setChangingChannel] = useState<string | null>(null)

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

  const handleApprove = async (id: string) => {
    setApproving(id)
    try {
      await api.approveAgent(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to approve agent')
    } finally {
      setApproving(null)
    }
  }

  const handleChannelChange = async (id: string, channel: string) => {
    setChangingChannel(id)
    try {
      await api.updateAgentChannel(id, channel)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to change update channel')
    } finally {
      setChangingChannel(null)
    }
  }

  const toggleSelect = (id: string) => {
    setSelected(s => {
      const n = new Set(s)
      if (n.has(id)) n.delete(id); else n.add(id)
      return n
    })
  }

  const pendingSelected = agents.filter(a => selected.has(a.id) && a.status === 'pending_approval')

  const allChecked = agents.length > 0 && agents.every(a => selected.has(a.id))
  const someChecked = agents.some(a => selected.has(a.id))

  const toggleAll = () => {
    if (allChecked) {
      setSelected(new Set())
    } else {
      setSelected(new Set(agents.map(a => a.id)))
    }
  }

  const handleBulkApprove = async () => {
    setBulkApproving(true)
    setError('')
    try {
      for (const a of pendingSelected) {
        await api.approveAgent(a.id)
      }
      setSelected(new Set())
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to bulk approve agents')
    } finally {
      setBulkApproving(false)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-2xl font-bold text-th-text">Agents</h1>
        {pendingSelected.length > 0 && (
          <button
            onClick={handleBulkApprove}
            disabled={bulkApproving}
            className="text-sm px-3 py-1.5 rounded font-medium disabled:opacity-50"
            style={{
              backgroundColor: 'var(--th-badge-success-bg)',
              color: 'var(--th-badge-success-text)',
            }}
          >
            {bulkApproving ? 'Approving…' : `Approve Selected (${pendingSelected.length})`}
          </button>
        )}
      </div>
      {error && <p className="text-red-600 text-sm">{error}</p>}

      {/* Desktop table */}
      <div className="hidden sm:block bg-th-surface rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-th-border text-sm">
          <thead className="bg-th-surface-muted">
            <tr>
              <th className="px-4 py-2 text-left">
                <input
                  type="checkbox"
                  checked={allChecked}
                  ref={el => { if (el) el.indeterminate = someChecked && !allChecked }}
                  onChange={toggleAll}
                  className="rounded border-th-input-border"
                />
              </th>
              {['Name', 'Hostname', 'IP', 'Status', 'CPU', 'RAM', 'GPU', 'Channel', 'Last Heartbeat', 'Tags', ''].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-th-border-subtle">
            {agents.map(a => (
              <Fragment key={a.id}>
              <tr className="hover:bg-th-surface-muted">
                <td className="px-4 py-2">
                  <input
                    type="checkbox"
                    checked={selected.has(a.id)}
                    onChange={() => toggleSelect(a.id)}
                    className="rounded border-th-input-border"
                  />
                </td>
                <td
                  className="px-4 py-2 font-medium text-th-text cursor-pointer hover:underline"
                  title="Click to toggle resource utilisation graph"
                  onClick={() => setExpandedMetrics(expandedMetrics === a.id ? null : a.id)}
                >
                  {a.name}
                  <span className="ml-1 text-xs text-th-text-muted">{expandedMetrics === a.id ? '▲' : '▼'}</span>
                </td>
                <td className="px-4 py-2 text-th-text-secondary">{a.hostname}</td>
                <td className="px-4 py-2 text-th-text-secondary">{a.ip_address}</td>
                <td className="px-4 py-2"><StatusBadge status={a.status} /></td>
                <td className="px-4 py-2 text-th-text-secondary">{a.cpu_count} cores</td>
                <td className="px-4 py-2 text-th-text-secondary">{fmtBytes(a.ram_mib)}</td>
                <td className="px-4 py-2 text-th-text-secondary">
                  {a.gpu_enabled ? (
                    <span title={[a.nvenc && 'NVENC', a.qsv && 'QSV', a.amf && 'AMF'].filter(Boolean).join(', ')}>
                      {a.gpu_vendor} {a.gpu_model}
                    </span>
                  ) : '—'}
                </td>
                <td className="px-4 py-2">
                  <div className="flex items-center gap-1.5">
                    <ChannelBadge channel={a.update_channel ?? 'stable'} />
                    <select
                      value={a.update_channel ?? 'stable'}
                      disabled={changingChannel === a.id}
                      onChange={e => handleChannelChange(a.id, e.target.value)}
                      className="text-xs border border-th-input-border rounded bg-th-surface text-th-text px-1 py-0.5"
                      title="Change update channel"
                    >
                      <option value="stable">stable</option>
                      <option value="beta">beta</option>
                      <option value="nightly">nightly</option>
                    </select>
                  </div>
                </td>
                <td className="px-4 py-2 text-th-text-muted whitespace-nowrap">{fmtDate(a.last_heartbeat)}</td>
                <td className="px-4 py-2 text-th-text-muted">
                  {a.tags.length > 0 ? a.tags.join(', ') : '—'}
                </td>
                <td className="px-4 py-2 flex gap-1 flex-wrap">
                  {(a.status === 'idle' || a.status === 'running') && (
                    <button
                      onClick={() => handleDrain(a.id)}
                      disabled={draining === a.id}
                      className="text-xs px-2 py-1 rounded disabled:opacity-50"
                      style={{
                        backgroundColor: 'var(--th-badge-draining-bg)',
                        color: 'var(--th-badge-draining-text)',
                      }}
                    >
                      {draining === a.id ? 'Draining…' : 'Drain'}
                    </button>
                  )}
                  {a.status === 'pending_approval' && (
                    <button
                      onClick={() => handleApprove(a.id)}
                      disabled={approving === a.id}
                      className="text-xs px-2 py-1 rounded disabled:opacity-50"
                      style={{
                        backgroundColor: 'var(--th-badge-success-bg)',
                        color: 'var(--th-badge-success-text)',
                      }}
                    >
                      {approving === a.id ? 'Approving…' : 'Approve'}
                    </button>
                  )}
                  {a.vnc_port > 0 && (
                    <a
                      href={`/novnc/${a.id}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-xs px-2 py-1 rounded"
                      title={`Open remote desktop (VNC port ${a.vnc_port})`}
                      style={{
                        backgroundColor: 'var(--th-badge-running-bg)',
                        color: 'var(--th-badge-running-text)',
                        textDecoration: 'none',
                      }}
                    >
                      Remote Desktop
                    </a>
                  )}
                </td>
              </tr>
              {expandedMetrics === a.id && (
                <tr className="bg-th-surface-muted">
                  <td colSpan={11} className="px-6 py-3">
                    <p className="text-xs font-medium text-th-text-muted mb-2 uppercase tracking-wide">
                      Resource Utilisation — last hour
                    </p>
                    <AgentMetricsGraph agentId={a.id} />
                  </td>
                </tr>
              )}
              </Fragment>
            ))}
            {agents.length === 0 && (
              <tr><td colSpan={11} className="px-4 py-4 text-center text-th-text-subtle">No agents registered</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Mobile card list */}
      <div className="sm:hidden space-y-3">
        {agents.map(a => (
          <div key={a.id} className="bg-th-surface rounded-lg shadow">
            {/* Card header */}
            <div className="px-4 py-3 flex items-center gap-3 border-b border-th-border-subtle">
              <input
                type="checkbox"
                checked={selected.has(a.id)}
                onChange={() => toggleSelect(a.id)}
                className="rounded border-th-input-border shrink-0"
              />
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium text-th-text truncate">{a.name}</span>
                  <StatusBadge status={a.status} />
                </div>
                <p className="text-xs text-th-text-muted mt-0.5">{a.hostname} · {a.ip_address}</p>
              </div>
            </div>

            {/* Card details */}
            <div className="px-4 py-2 grid grid-cols-2 gap-1 text-xs">
              <div><span className="text-th-text-muted">CPU: </span><span className="text-th-text-secondary">{a.cpu_count} cores</span></div>
              <div><span className="text-th-text-muted">RAM: </span><span className="text-th-text-secondary">{fmtBytes(a.ram_mib)}</span></div>
              {a.gpu_enabled && (
                <div className="col-span-2"><span className="text-th-text-muted">GPU: </span><span className="text-th-text-secondary">{a.gpu_vendor} {a.gpu_model}</span></div>
              )}
              {a.tags.length > 0 && (
                <div className="col-span-2"><span className="text-th-text-muted">Tags: </span><span className="text-th-text-secondary">{a.tags.join(', ')}</span></div>
              )}
              <div className="col-span-2"><span className="text-th-text-muted">Heartbeat: </span><span className="text-th-text-secondary">{fmtDate(a.last_heartbeat)}</span></div>
              <div className="col-span-2 flex items-center gap-1.5 mt-0.5">
                <span className="text-th-text-muted">Channel: </span>
                <ChannelBadge channel={a.update_channel ?? 'stable'} />
                <select
                  value={a.update_channel ?? 'stable'}
                  disabled={changingChannel === a.id}
                  onChange={e => handleChannelChange(a.id, e.target.value)}
                  className="text-xs border border-th-input-border rounded bg-th-surface text-th-text px-1 py-0.5"
                  title="Change update channel"
                >
                  <option value="stable">stable</option>
                  <option value="beta">beta</option>
                  <option value="nightly">nightly</option>
                </select>
              </div>
            </div>

            {/* Card actions */}
            <div className="px-4 py-2 flex gap-2 flex-wrap border-t border-th-border-subtle">
              <button
                onClick={() => setExpandedMetrics(expandedMetrics === a.id ? null : a.id)}
                className="text-xs px-2 py-1 rounded border border-th-input-border text-th-text-muted hover:bg-th-surface-muted"
              >
                {expandedMetrics === a.id ? 'Hide Metrics' : 'Show Metrics'}
              </button>
              {(a.status === 'idle' || a.status === 'running') && (
                <button
                  onClick={() => handleDrain(a.id)}
                  disabled={draining === a.id}
                  className="text-xs px-2 py-1 rounded disabled:opacity-50"
                  style={{ backgroundColor: 'var(--th-badge-draining-bg)', color: 'var(--th-badge-draining-text)' }}
                >
                  {draining === a.id ? 'Draining…' : 'Drain'}
                </button>
              )}
              {a.status === 'pending_approval' && (
                <button
                  onClick={() => handleApprove(a.id)}
                  disabled={approving === a.id}
                  className="text-xs px-2 py-1 rounded disabled:opacity-50"
                  style={{ backgroundColor: 'var(--th-badge-success-bg)', color: 'var(--th-badge-success-text)' }}
                >
                  {approving === a.id ? 'Approving…' : 'Approve'}
                </button>
              )}
              {a.vnc_port > 0 && (
                <a
                  href={`/novnc/${a.id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-xs px-2 py-1 rounded"
                  style={{ backgroundColor: 'var(--th-badge-running-bg)', color: 'var(--th-badge-running-text)', textDecoration: 'none' }}
                >
                  Remote Desktop
                </a>
              )}
            </div>

            {/* Metrics expansion */}
            {expandedMetrics === a.id && (
              <div className="px-4 py-3 border-t border-th-border-subtle bg-th-surface-muted">
                <p className="text-xs font-medium text-th-text-muted mb-2 uppercase tracking-wide">
                  Resource Utilisation — last hour
                </p>
                <AgentMetricsGraph agentId={a.id} />
              </div>
            )}
          </div>
        ))}
        {agents.length === 0 && (
          <p className="text-center text-th-text-subtle text-sm py-8">No agents registered</p>
        )}
      </div>
    </div>
  )
}
