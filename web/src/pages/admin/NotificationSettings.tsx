import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { NotificationPrefs } from '../../types'

const DEFAULT_PREFS: NotificationPrefs = {
  id: '',
  user_id: '',
  notify_on_job_complete: true,
  notify_on_job_failed: true,
  notify_on_agent_stale: false,
  webhook_filter_user_only: false,
  email_address: '',
  notify_email: false,
  created_at: '',
  updated_at: '',
}

export default function NotificationSettings() {
  const [prefs, setPrefs] = useState<NotificationPrefs>(DEFAULT_PREFS)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [testResult, setTestResult] = useState('')
  const [channelTesting, setChannelTesting] = useState<string | null>(null)
  const [channelTestResult, setChannelTestResult] = useState<Record<string, string>>({})

  const load = useCallback(async () => {
    try {
      const data = await api.getNotificationPrefs()
      setPrefs(data)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load notification preferences')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleSave = async () => {
    setSaving(true)
    setError('')
    setSuccess('')
    try {
      const updated = await api.updateNotificationPrefs({
        notify_on_job_complete: prefs.notify_on_job_complete,
        notify_on_job_failed: prefs.notify_on_job_failed,
        notify_on_agent_stale: prefs.notify_on_agent_stale,
        webhook_filter_user_only: prefs.webhook_filter_user_only,
        email_address: prefs.email_address,
        notify_email: prefs.notify_email,
      })
      setPrefs(updated)
      setSuccess('Preferences saved.')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save preferences')
    } finally {
      setSaving(false)
    }
  }

  const handleTestEmail = async () => {
    if (!prefs.email_address) {
      setTestResult('Enter an email address first.')
      return
    }
    setTesting(true)
    setTestResult('')
    try {
      const result = await api.testEmail(prefs.email_address)
      setTestResult(result.ok ? `Test email sent to ${result.to}.` : 'Test email failed.')
    } catch (e: unknown) {
      setTestResult(e instanceof Error ? e.message : 'Test email failed.')
    } finally {
      setTesting(false)
    }
  }

  const toggle = (field: keyof NotificationPrefs) => {
    setPrefs(prev => ({ ...prev, [field]: !prev[field] }))
    setSuccess('')
  }

  const handleTestChannel = async (channel: 'telegram' | 'pushover' | 'ntfy') => {
    setChannelTesting(channel)
    setChannelTestResult(prev => ({ ...prev, [channel]: '' }))
    try {
      let result: { ok: boolean }
      if (channel === 'telegram') result = await api.testTelegram()
      else if (channel === 'pushover') result = await api.testPushover()
      else result = await api.testNtfy()
      setChannelTestResult(prev => ({
        ...prev,
        [channel]: result.ok ? 'Test message sent.' : 'Test failed.',
      }))
    } catch (e: unknown) {
      setChannelTestResult(prev => ({
        ...prev,
        [channel]: e instanceof Error ? e.message : 'Test failed.',
      }))
    } finally {
      setChannelTesting(null)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-bold text-th-text">Notification Settings</h1>
        <p className="text-sm text-th-text-muted mt-0.5">
          Configure which events trigger in-app and email notifications.
        </p>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}
      {success && <p className="text-green-600 text-sm">{success}</p>}

      {/* Webhook / in-app notifications */}
      <section className="bg-th-surface rounded-lg shadow p-5 space-y-4">
        <h2 className="text-sm font-semibold text-th-text">Event Notifications</h2>

        <CheckboxRow
          label="Job completed"
          description="Notify when a job finishes successfully."
          checked={prefs.notify_on_job_complete}
          onChange={() => toggle('notify_on_job_complete')}
        />
        <CheckboxRow
          label="Job failed"
          description="Notify when a job fails."
          checked={prefs.notify_on_job_failed}
          onChange={() => toggle('notify_on_job_failed')}
        />
        <CheckboxRow
          label="Agent stale"
          description="Notify when an agent stops sending heartbeats."
          checked={prefs.notify_on_agent_stale}
          onChange={() => toggle('notify_on_agent_stale')}
        />
        <CheckboxRow
          label="Show only my jobs"
          description="Only receive webhook notifications for jobs you created."
          checked={prefs.webhook_filter_user_only}
          onChange={() => toggle('webhook_filter_user_only')}
        />
      </section>

      {/* Email notifications */}
      <section className="bg-th-surface rounded-lg shadow p-5 space-y-4">
        <h2 className="text-sm font-semibold text-th-text">Email Notifications</h2>

        <CheckboxRow
          label="Enable email notifications"
          description="Send an email for events that are enabled above."
          checked={prefs.notify_email}
          onChange={() => toggle('notify_email')}
        />

        <div className="space-y-1">
          <label className="block text-sm font-medium text-th-text" htmlFor="email-address">
            Email address
          </label>
          <div className="flex gap-2">
            <input
              id="email-address"
              type="email"
              value={prefs.email_address}
              onChange={e => { setPrefs(prev => ({ ...prev, email_address: e.target.value })); setSuccess('') }}
              placeholder="you@example.com"
              className="flex-1 rounded border border-th-border bg-th-input px-3 py-1.5 text-sm text-th-text placeholder:text-th-text-subtle focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
            <button
              onClick={handleTestEmail}
              disabled={testing || !prefs.email_address}
              className="shrink-0 rounded bg-th-surface-muted border border-th-border px-3 py-1.5 text-sm text-th-text hover:bg-th-surface disabled:opacity-50"
            >
              {testing ? 'Sending…' : 'Test'}
            </button>
          </div>
          {testResult && (
            <p className={`text-xs mt-1 ${testResult.includes('sent') ? 'text-green-600' : 'text-red-500'}`}>
              {testResult}
            </p>
          )}
          <p className="text-xs text-th-text-muted">
            Requires SMTP to be configured on the controller. Use the Test button to verify delivery.
          </p>
        </div>
      </section>

      {/* Telegram */}
      <section className="bg-th-surface rounded-lg shadow p-5 space-y-3">
        <h2 className="text-sm font-semibold text-th-text">Telegram</h2>
        <p className="text-xs text-th-text-muted">
          Configure <code>notifications.telegram.bot_token</code> and <code>chat_id</code> in the controller config to enable.
        </p>
        <div className="flex items-center gap-3">
          <button
            onClick={() => handleTestChannel('telegram')}
            disabled={channelTesting === 'telegram'}
            className="rounded bg-th-surface-muted border border-th-border px-3 py-1.5 text-sm text-th-text hover:bg-th-surface disabled:opacity-50"
          >
            {channelTesting === 'telegram' ? 'Sending…' : 'Send Test Message'}
          </button>
          {channelTestResult.telegram && (
            <p className={`text-xs ${channelTestResult.telegram.includes('sent') ? 'text-green-600' : 'text-red-500'}`}>
              {channelTestResult.telegram}
            </p>
          )}
        </div>
      </section>

      {/* Pushover */}
      <section className="bg-th-surface rounded-lg shadow p-5 space-y-3">
        <h2 className="text-sm font-semibold text-th-text">Pushover</h2>
        <p className="text-xs text-th-text-muted">
          Configure <code>notifications.pushover.app_token</code> and <code>user_key</code> in the controller config to enable.
        </p>
        <div className="flex items-center gap-3">
          <button
            onClick={() => handleTestChannel('pushover')}
            disabled={channelTesting === 'pushover'}
            className="rounded bg-th-surface-muted border border-th-border px-3 py-1.5 text-sm text-th-text hover:bg-th-surface disabled:opacity-50"
          >
            {channelTesting === 'pushover' ? 'Sending…' : 'Send Test Message'}
          </button>
          {channelTestResult.pushover && (
            <p className={`text-xs ${channelTestResult.pushover.includes('sent') ? 'text-green-600' : 'text-red-500'}`}>
              {channelTestResult.pushover}
            </p>
          )}
        </div>
      </section>

      {/* ntfy */}
      <section className="bg-th-surface rounded-lg shadow p-5 space-y-3">
        <h2 className="text-sm font-semibold text-th-text">ntfy</h2>
        <p className="text-xs text-th-text-muted">
          Configure <code>notifications.ntfy.topic</code> (and optionally <code>server_url</code> and <code>token</code>) in the controller config to enable.
        </p>
        <div className="flex items-center gap-3">
          <button
            onClick={() => handleTestChannel('ntfy')}
            disabled={channelTesting === 'ntfy'}
            className="rounded bg-th-surface-muted border border-th-border px-3 py-1.5 text-sm text-th-text hover:bg-th-surface disabled:opacity-50"
          >
            {channelTesting === 'ntfy' ? 'Sending…' : 'Send Test Message'}
          </button>
          {channelTestResult.ntfy && (
            <p className={`text-xs ${channelTestResult.ntfy.includes('sent') ? 'text-green-600' : 'text-red-500'}`}>
              {channelTestResult.ntfy}
            </p>
          )}
        </div>
      </section>

      <div className="flex justify-end">
        <button
          onClick={handleSave}
          disabled={saving}
          className="rounded bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {saving ? 'Saving…' : 'Save Preferences'}
        </button>
      </div>
    </div>
  )
}

function CheckboxRow({
  label,
  description,
  checked,
  onChange,
}: {
  label: string
  description: string
  checked: boolean
  onChange: () => void
}) {
  return (
    <label className="flex items-start gap-3 cursor-pointer select-none">
      <input
        type="checkbox"
        checked={checked}
        onChange={onChange}
        className="mt-0.5 h-4 w-4 rounded border-th-border text-blue-600 focus:ring-blue-500"
      />
      <div className="min-w-0">
        <p className="text-sm font-medium text-th-text">{label}</p>
        <p className="text-xs text-th-text-muted">{description}</p>
      </div>
    </label>
  )
}
