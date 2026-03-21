import { useState, useEffect, useCallback } from 'react'
import * as api from '../api/client'
import type { AgentMetric } from '../api/client'

interface Props {
  agentId: string
}

const WIDTH = 480
const HEIGHT = 120
const PAD = { top: 8, right: 8, bottom: 24, left: 32 }

const SERIES = [
  { key: 'cpu_pct' as keyof AgentMetric, label: 'CPU', color: 'var(--th-badge-running-text, #3b82f6)' },
  { key: 'gpu_pct' as keyof AgentMetric, label: 'GPU', color: 'var(--th-badge-success-text, #22c55e)' },
  { key: 'mem_pct' as keyof AgentMetric, label: 'Mem', color: 'var(--th-badge-draining-text, #f59e0b)' },
]

function buildPath(points: { x: number; y: number }[]): string {
  if (points.length === 0) return ''
  return points
    .map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`)
    .join(' ')
}

export default function AgentMetricsGraph({ agentId }: Props) {
  const [metrics, setMetrics] = useState<AgentMetric[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    try {
      const data = await api.listAgentMetrics(agentId, '1h')
      setMetrics(data ?? [])
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load metrics')
    } finally {
      setLoading(false)
    }
  }, [agentId])

  useEffect(() => {
    load()
    const id = setInterval(load, 30_000)
    return () => clearInterval(id)
  }, [load])

  if (loading) return <p className="text-xs text-th-text-muted">Loading metrics…</p>
  if (error) return <p className="text-xs text-red-500">{error}</p>
  if (metrics.length === 0) {
    return <p className="text-xs text-th-text-subtle">No metric samples recorded yet.</p>
  }

  const innerW = WIDTH - PAD.left - PAD.right
  const innerH = HEIGHT - PAD.top - PAD.bottom

  // x scale: map first→last timestamp to 0→innerW
  const tMin = new Date(metrics[0].recorded_at).getTime()
  const tMax = new Date(metrics[metrics.length - 1].recorded_at).getTime()
  const tRange = tMax - tMin || 1

  function toX(ts: string) {
    return PAD.left + ((new Date(ts).getTime() - tMin) / tRange) * innerW
  }

  function toY(val: number) {
    // y=0 at top, 100% at bottom
    return PAD.top + innerH - (val / 100) * innerH
  }

  // Y-axis tick labels
  const yTicks = [0, 25, 50, 75, 100]

  return (
    <div>
      <div className="flex items-center gap-4 mb-1">
        {SERIES.map(s => (
          <span key={s.key} className="flex items-center gap-1 text-xs text-th-text-secondary">
            <span
              style={{
                display: 'inline-block',
                width: 10,
                height: 2,
                backgroundColor: s.color,
                borderRadius: 1,
              }}
            />
            {s.label}
          </span>
        ))}
      </div>
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        width={WIDTH}
        height={HEIGHT}
        style={{ display: 'block', maxWidth: '100%', overflow: 'visible' }}
        aria-label="Agent resource utilisation over the last hour"
      >
        {/* Grid lines + Y-axis labels */}
        {yTicks.map(pct => {
          const y = toY(pct)
          return (
            <g key={pct}>
              <line
                x1={PAD.left}
                y1={y}
                x2={PAD.left + innerW}
                y2={y}
                stroke="currentColor"
                strokeOpacity={0.1}
                strokeWidth={1}
              />
              <text
                x={PAD.left - 4}
                y={y + 4}
                fontSize={9}
                textAnchor="end"
                fill="currentColor"
                opacity={0.4}
              >
                {pct}%
              </text>
            </g>
          )
        })}

        {/* Series lines */}
        {SERIES.map(s => {
          const pts = metrics.map(m => ({
            x: toX(m.recorded_at),
            y: toY(m[s.key] as number),
          }))
          return (
            <path
              key={s.key}
              d={buildPath(pts)}
              fill="none"
              stroke={s.color}
              strokeWidth={1.5}
              strokeLinejoin="round"
              strokeLinecap="round"
            />
          )
        })}

        {/* X-axis baseline */}
        <line
          x1={PAD.left}
          y1={PAD.top + innerH}
          x2={PAD.left + innerW}
          y2={PAD.top + innerH}
          stroke="currentColor"
          strokeOpacity={0.15}
          strokeWidth={1}
        />
      </svg>
    </div>
  )
}
