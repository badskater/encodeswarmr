import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { Plugin } from '../../types'
import { useAutoRefresh } from '../../hooks/useAutoRefresh'

// Mock data used as a fallback when the backend plugin endpoint is not yet
// wired up. The UI shell is intentionally functional so operators can review
// the layout before the backend plugin system is implemented.
const MOCK_PLUGINS: Plugin[] = [
  {
    id: 'x265-encoder',
    name: 'x265 Encoder',
    version: '3.5.0',
    description: 'HEVC/H.265 encoding via libx265. Supports 8-bit and 10-bit output.',
    enabled: true,
    author: 'VideoLAN',
  },
  {
    id: 'av1-svt',
    name: 'SVT-AV1 Encoder',
    version: '1.8.0',
    description: 'Scalable Video Technology AV1 encoder. High efficiency, multi-threaded.',
    enabled: true,
    author: 'Alliance for Open Media',
  },
  {
    id: 'dolby-vision-injector',
    name: 'Dolby Vision Metadata Injector',
    version: '2.1.0',
    description: 'Injects Dolby Vision RPU metadata into encoded streams using dovi_tool.',
    enabled: false,
    author: null,
  },
  {
    id: 'vmaf-reporter',
    name: 'VMAF Quality Reporter',
    version: '0.9.2',
    description: 'Computes VMAF scores after encode and attaches them to the job result.',
    enabled: true,
    author: 'Netflix OSS',
  },
  {
    id: 'scene-detect-pyscene',
    name: 'PySceneDetect',
    version: '0.6.3',
    description: 'Scene detection using content-aware and threshold-based algorithms.',
    enabled: false,
    author: null,
  },
]

function PluginCard({
  plugin,
  onToggle,
  toggling,
}: {
  plugin: Plugin
  onToggle: (name: string, enable: boolean) => void
  toggling: string | null
}) {
  const isToggling = toggling === plugin.name

  return (
    <div className="bg-th-surface rounded-lg shadow p-4 flex flex-col gap-3 sm:flex-row sm:items-start sm:gap-4">
      {/* Plugin icon placeholder */}
      <div
        className="shrink-0 w-10 h-10 rounded-lg flex items-center justify-center text-lg font-bold"
        style={{
          backgroundColor: plugin.enabled ? 'var(--th-badge-success-bg)' : 'var(--th-badge-neutral-bg)',
          color: plugin.enabled ? 'var(--th-badge-success-text)' : 'var(--th-badge-neutral-text)',
        }}
        aria-hidden
      >
        {plugin.name.slice(0, 1).toUpperCase()}
      </div>

      {/* Plugin info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-th-text">{plugin.name}</h3>
            <p className="text-xs text-th-text-muted mt-0.5">
              v{plugin.version}
              {plugin.author && <> · {plugin.author}</>}
            </p>
          </div>

          {/* Enable/disable toggle */}
          <button
            onClick={() => onToggle(plugin.name, !plugin.enabled)}
            disabled={isToggling}
            className={`shrink-0 relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus:outline-none disabled:opacity-50 ${
              plugin.enabled ? 'bg-blue-600' : 'bg-th-border'
            }`}
            aria-label={plugin.enabled ? 'Disable plugin' : 'Enable plugin'}
            role="switch"
            aria-checked={plugin.enabled}
          >
            <span
              className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform ${
                plugin.enabled ? 'translate-x-4' : 'translate-x-0.5'
              }`}
            />
          </button>
        </div>

        <p className="text-sm text-th-text-secondary mt-1.5 leading-relaxed">{plugin.description}</p>

        <div className="flex items-center gap-2 mt-2">
          <span
            className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium"
            style={{
              backgroundColor: plugin.enabled ? 'var(--th-badge-success-bg)' : 'var(--th-badge-neutral-bg)',
              color: plugin.enabled ? 'var(--th-badge-success-text)' : 'var(--th-badge-neutral-text)',
            }}
          >
            {plugin.enabled ? 'Enabled' : 'Disabled'}
          </span>
        </div>
      </div>
    </div>
  )
}

export default function Plugins() {
  const [plugins, setPlugins] = useState<Plugin[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [toggling, setToggling] = useState<string | null>(null)
  const [useMock, setUseMock] = useState(false)

  const load = useCallback(async () => {
    try {
      const data = await api.listPlugins()
      setPlugins(data)
      setUseMock(false)
      setError('')
    } catch {
      // Backend plugin endpoint not yet implemented — fall back to mock data
      // so the UI shell remains usable.
      setPlugins(MOCK_PLUGINS)
      setUseMock(true)
      setError('')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])
  useAutoRefresh(load)

  const handleToggle = async (name: string, enable: boolean) => {
    setToggling(name)
    try {
      if (useMock) {
        // Optimistic update on mock data
        setPlugins(prev => prev.map(p => p.name === name ? { ...p, enabled: enable } : p))
      } else {
        const updated = await api.togglePlugin(name, enable)
        setPlugins(prev => prev.map(p => p.name === updated.name ? updated : p))
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to update plugin')
    } finally {
      setToggling(null)
    }
  }

  const enabledCount = plugins.filter(p => p.enabled).length

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold text-th-text">Plugins</h1>
          <p className="text-sm text-th-text-muted mt-0.5">
            Manage encoding plugins · {enabledCount} of {plugins.length} enabled
          </p>
        </div>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {useMock && (
        <div className="rounded border border-th-border-subtle bg-th-surface-muted px-4 py-3 text-sm text-th-text-muted">
          Plugin backend not yet connected — showing example plugins. Toggle state is local only.
        </div>
      )}

      <div className="space-y-3">
        {plugins.map(plugin => (
          <PluginCard
            key={plugin.id}
            plugin={plugin}
            onToggle={handleToggle}
            toggling={toggling}
          />
        ))}
        {plugins.length === 0 && (
          <p className="text-center text-th-text-subtle text-sm py-8">No plugins installed</p>
        )}
      </div>
    </div>
  )
}
