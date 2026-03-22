import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { AutoScalingSettings } from '../../types'
import { useAutoRefresh } from '../../hooks/useAutoRefresh'

const DEFAULT_SETTINGS: AutoScalingSettings = {
  enabled: false,
  webhook_url: '',
  scale_up_threshold: 10,
  scale_down_threshold: 2,
  cooldown_seconds: 300,
}

export default function AutoScaling() {
  const [settings, setSettings] = useState<AutoScalingSettings>(DEFAULT_SETTINGS)
  const [queueDepth, setQueueDepth] = useState<number | null>(null)
  const [idleAgents, setIdleAgents] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [testResult, setTestResult] = useState('')

  const loadSettings = useCallback(async () => {
    try {
      const data = await api.getAutoScaling()
      setSettings(data)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load auto-scaling settings')
    } finally {
      setLoading(false)
    }
  }, [])

  const loadStatus = useCallback(async () => {
    try {
      const [queue, agents] = await Promise.all([
        api.getQueueSummary(),
        api.listAgents(),
      ])
      setQueueDepth(queue.pending)
      setIdleAgents(agents.filter(a => a.status === 'idle').length)
    } catch {
      // Status is informational — don't block the page on errors.
    }
  }, [])

  const load = useCallback(async () => {
    await Promise.all([loadSettings(), loadStatus()])
  }, [loadSettings, loadStatus])

  useEffect(() => { load() }, [load])
  useAutoRefresh(loadStatus)

  const handleSave = async () => {
    setSaving(true)
    setError('')
    setSuccess('')
    try {
      const updated = await api.updateAutoScaling(settings)
      setSettings(updated)
      setSuccess('Auto-scaling settings saved.')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async () => {
    if (!settings.webhook_url) {
      setTestResult('Enter a webhook URL first.')
      return
    }
    setTesting(true)
    setTestResult('')
    try {
      const result = await api.testAutoScalingWebhook()
      setTestResult(result.ok ? `Test payload sent to ${result.url}.` : 'Webhook test failed.')
    } catch (e: unknown) {
      setTestResult(e instanceof Error ? e.message : 'Webhook test failed.')
    } finally {
      setTesting(false)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-6 max-w-2xl">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold text-th-text">Auto-Scaling</h1>
          <p className="text-sm text-th-text-muted mt-0.5">
            Fire webhook events when queue depth or idle agent count crosses thresholds.
          </p>
        </div>
        {/* Current status badge */}
        <span
          className="mt-1 shrink-0 inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium"
          style={{
            backgroundColor: settings.enabled ? 'var(--th-badge-success-bg)' : 'var(--th-badge-neutral-bg)',
            color: settings.enabled ? 'var(--th-badge-success-text)' : 'var(--th-badge-neutral-text)',
          }}
        >
          {settings.enabled ? 'Enabled' : 'Disabled'}
        </span>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}
      {success && <p className="text-green-600 text-sm">{success}</p>}

      {/* Current status */}
      <section className="bg-th-surface rounded-lg shadow p-5">
        <h2 className="text-sm font-semibold text-th-text mb-3">Current Status</h2>
        <div className="grid grid-cols-2 gap-4">
          <StatusCard
            label="Queue Depth"
            value={queueDepth !== null ? String(queueDepth) : '—'}
            sublabel="pending tasks"
            highlight={
              queueDepth !== null && queueDepth > settings.scale_up_threshold
                ? 'high'
                : null
            }
          />
          <StatusCard
            label="Idle Agents"
            value={idleAgents !== null ? String(idleAgents) : '—'}
            sublabel="waiting for work"
            highlight={
              idleAgents !== null && idleAgents > settings.scale_down_threshold
                ? 'low'
                : null
            }
          />
        </div>
      </section>

      {/* Configuration */}
      <section className="bg-th-surface rounded-lg shadow p-5 space-y-5">
        <h2 className="text-sm font-semibold text-th-text">Configuration</h2>

        {/* Enable toggle */}
        <label className="flex items-center gap-3 cursor-pointer select-none">
          <button
            role="switch"
            aria-checked={settings.enabled}
            onClick={() => { setSettings(prev => ({ ...prev, enabled: !prev.enabled })); setSuccess('') }}
            className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus:outline-none ${
              settings.enabled ? 'bg-blue-600' : 'bg-th-border'
            }`}
          >
            <span
              className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform ${
                settings.enabled ? 'translate-x-4' : 'translate-x-0.5'
              }`}
            />
          </button>
          <div>
            <p className="text-sm font-medium text-th-text">Enable auto-scaling</p>
            <p className="text-xs text-th-text-muted">Fire webhooks when thresholds are crossed.</p>
          </div>
        </label>

        {/* Webhook URL */}
        <div className="space-y-1">
          <label className="block text-sm font-medium text-th-text" htmlFor="as-webhook-url">
            Webhook URL
          </label>
          <div className="flex gap-2">
            <input
              id="as-webhook-url"
              type="url"
              value={settings.webhook_url}
              onChange={e => { setSettings(prev => ({ ...prev, webhook_url: e.target.value })); setSuccess('') }}
              placeholder="https://your-scaler/hook"
              className="flex-1 rounded border border-th-border bg-th-input px-3 py-1.5 text-sm text-th-text placeholder:text-th-text-subtle focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
            <button
              onClick={handleTest}
              disabled={testing || !settings.webhook_url}
              className="shrink-0 rounded bg-th-surface-muted border border-th-border px-3 py-1.5 text-sm text-th-text hover:bg-th-surface disabled:opacity-50"
            >
              {testing ? 'Testing…' : 'Test Webhook'}
            </button>
          </div>
          {testResult && (
            <p className={`text-xs mt-1 ${testResult.includes('sent') ? 'text-green-600' : 'text-red-500'}`}>
              {testResult}
            </p>
          )}
          <p className="text-xs text-th-text-muted">
            Receives a JSON POST with <code className="font-mono">action</code>,{' '}
            <code className="font-mono">pending_tasks</code>, <code className="font-mono">active_agents</code>, and{' '}
            <code className="font-mono">idle_agents</code> fields.
          </p>
        </div>

        {/* Scale-up threshold */}
        <div className="space-y-1">
          <div className="flex items-center justify-between">
            <label className="block text-sm font-medium text-th-text" htmlFor="as-scale-up">
              Scale-up threshold
            </label>
            <span className="text-sm font-semibold text-th-text tabular-nums">
              {settings.scale_up_threshold} tasks
            </span>
          </div>
          <input
            id="as-scale-up"
            type="range"
            min={1}
            max={100}
            step={1}
            value={settings.scale_up_threshold}
            onChange={e => { setSettings(prev => ({ ...prev, scale_up_threshold: Number(e.target.value) })); setSuccess('') }}
            className="w-full accent-blue-600"
          />
          <p className="text-xs text-th-text-muted">
            Fire <code className="font-mono">scale_up</code> when pending tasks exceed this value and no agents are idle.
          </p>
        </div>

        {/* Scale-down threshold */}
        <div className="space-y-1">
          <div className="flex items-center justify-between">
            <label className="block text-sm font-medium text-th-text" htmlFor="as-scale-down">
              Scale-down threshold
            </label>
            <span className="text-sm font-semibold text-th-text tabular-nums">
              {settings.scale_down_threshold} idle agents
            </span>
          </div>
          <input
            id="as-scale-down"
            type="range"
            min={0}
            max={20}
            step={1}
            value={settings.scale_down_threshold}
            onChange={e => { setSettings(prev => ({ ...prev, scale_down_threshold: Number(e.target.value) })); setSuccess('') }}
            className="w-full accent-blue-600"
          />
          <p className="text-xs text-th-text-muted">
            Fire <code className="font-mono">scale_down</code> when idle agents exceed this count.
          </p>
        </div>

        {/* Cooldown */}
        <div className="space-y-1">
          <label className="block text-sm font-medium text-th-text" htmlFor="as-cooldown">
            Cooldown period (seconds)
          </label>
          <input
            id="as-cooldown"
            type="number"
            min={0}
            step={30}
            value={settings.cooldown_seconds}
            onChange={e => { setSettings(prev => ({ ...prev, cooldown_seconds: Number(e.target.value) })); setSuccess('') }}
            className="w-32 rounded border border-th-border bg-th-input px-3 py-1.5 text-sm text-th-text focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <p className="text-xs text-th-text-muted">
            Minimum seconds between successive scale events of the same type.
          </p>
        </div>
      </section>

      <div className="flex justify-end">
        <button
          onClick={handleSave}
          disabled={saving}
          className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {saving ? 'Saving…' : 'Save Settings'}
        </button>
      </div>
    </div>
  )
}

function StatusCard({
  label,
  value,
  sublabel,
  highlight,
}: {
  label: string
  value: string
  sublabel: string
  highlight: 'high' | 'low' | null
}) {
  const bgStyle = highlight === 'high'
    ? { backgroundColor: 'var(--th-badge-danger-bg)', color: 'var(--th-badge-danger-text)' }
    : highlight === 'low'
    ? { backgroundColor: 'var(--th-badge-warning-bg)', color: 'var(--th-badge-warning-text)' }
    : {}

  return (
    <div
      className="rounded-lg border border-th-border-subtle bg-th-surface-muted p-4"
      style={bgStyle}
    >
      <p className="text-xs text-th-text-muted">{label}</p>
      <p className="text-2xl font-bold text-th-text tabular-nums mt-0.5">{value}</p>
      <p className="text-xs text-th-text-subtle">{sublabel}</p>
    </div>
  )
}
