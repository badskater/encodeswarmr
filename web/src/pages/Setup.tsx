import { useState } from 'react'
import * as api from '../api/client'

type Step = 'account' | 'db-test'

interface Props {
  onComplete: () => void
}

export default function Setup({ onComplete }: Props) {
  const [step, setStep] = useState<Step>('account')
  const [form, setForm] = useState({ username: '', email: '', password: '', confirm: '' })
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [dbStatus, setDbStatus] = useState<'idle' | 'ok' | 'fail'>('idle')
  const [testing, setTesting] = useState(false)

  const handleCreateAccount = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (form.password !== form.confirm) {
      setError('Passwords do not match.')
      return
    }
    setSaving(true)
    try {
      const resp = await api.postSetup({
        username: form.username,
        email: form.email,
        password: form.password,
      })
      if (!resp.ok) {
        const body = await resp.json().catch(() => ({}))
        setError((body as { detail?: string }).detail ?? 'Setup failed. Please try again.')
        return
      }
      setStep('db-test')
    } catch {
      setError('Setup failed. Please try again.')
    } finally {
      setSaving(false)
    }
  }

  const handleTestDB = async () => {
    setTesting(true)
    setDbStatus('idle')
    try {
      const resp = await fetch('/health', { credentials: 'include' })
      setDbStatus(resp.ok ? 'ok' : 'fail')
    } catch {
      setDbStatus('fail')
    } finally {
      setTesting(false)
    }
  }

  const steps: { key: Step; label: string }[] = [
    { key: 'account', label: 'Create admin account' },
    { key: 'db-test', label: 'Test database connection' },
  ]

  return (
    <div className="min-h-screen bg-th-bg flex items-center justify-center px-4">
      <div className="bg-th-surface rounded-lg shadow p-8 w-full max-w-md">
        <h1 className="text-2xl font-bold text-th-text mb-1 text-center">
          EncodeSwarmr Setup
        </h1>
        <p className="text-sm text-th-text-muted text-center mb-6">
          Welcome. Complete these steps to get started.
        </p>

        {/* Step indicators */}
        <div className="flex items-center justify-center gap-6 mb-8">
          {steps.map((s, i) => {
            const done = (step === 'db-test' && s.key === 'account')
            const active = step === s.key
            return (
              <div key={s.key} className="flex items-center gap-2">
                <div
                  className={`w-7 h-7 rounded-full flex items-center justify-center text-sm font-bold shrink-0 ${
                    done
                      ? 'bg-green-500 text-white'
                      : active
                      ? 'bg-blue-600 text-white'
                      : 'bg-th-surface-muted text-th-text-muted'
                  }`}
                >
                  {done ? '✓' : i + 1}
                </div>
                <span className={`text-sm ${active ? 'text-th-text font-medium' : 'text-th-text-muted'}`}>
                  {s.label}
                </span>
              </div>
            )
          })}
        </div>

        {step === 'account' && (
          <form onSubmit={handleCreateAccount} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-th-text-secondary mb-1">Username</label>
              <input
                type="text"
                value={form.username}
                onChange={e => setForm(f => ({ ...f, username: e.target.value }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-3 py-2 text-sm text-th-text focus:outline-none focus:ring-2 focus:ring-blue-500"
                required
                autoFocus
                autoComplete="username"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-th-text-secondary mb-1">Email</label>
              <input
                type="email"
                value={form.email}
                onChange={e => setForm(f => ({ ...f, email: e.target.value }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-3 py-2 text-sm text-th-text focus:outline-none focus:ring-2 focus:ring-blue-500"
                required
                autoComplete="email"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-th-text-secondary mb-1">Password</label>
              <input
                type="password"
                value={form.password}
                onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-3 py-2 text-sm text-th-text focus:outline-none focus:ring-2 focus:ring-blue-500"
                required
                minLength={8}
                autoComplete="new-password"
              />
              <p className="text-xs text-th-text-subtle mt-1">Minimum 8 characters.</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-th-text-secondary mb-1">
                Confirm Password
              </label>
              <input
                type="password"
                value={form.confirm}
                onChange={e => setForm(f => ({ ...f, confirm: e.target.value }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-3 py-2 text-sm text-th-text focus:outline-none focus:ring-2 focus:ring-blue-500"
                required
                minLength={8}
                autoComplete="new-password"
              />
            </div>
            {error && <p className="text-red-600 text-sm">{error}</p>}
            <button
              type="submit"
              disabled={saving}
              className="w-full bg-blue-600 text-white rounded px-4 py-2 text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
            >
              {saving ? 'Creating account…' : 'Create Admin Account'}
            </button>
          </form>
        )}

        {step === 'db-test' && (
          <div className="space-y-4">
            <div className="bg-green-50 border border-green-200 rounded px-4 py-3">
              <p className="text-sm text-green-800 font-medium">Admin account created successfully.</p>
            </div>
            <p className="text-sm text-th-text-muted">
              Verify that the database connection is healthy before logging in.
            </p>
            <button
              onClick={handleTestDB}
              disabled={testing}
              className="w-full bg-blue-600 text-white rounded px-4 py-2 text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
            >
              {testing ? 'Testing…' : 'Test Database Connection'}
            </button>
            {dbStatus === 'ok' && (
              <p className="text-green-600 text-sm font-medium text-center">
                Database connection is healthy.
              </p>
            )}
            {dbStatus === 'fail' && (
              <p className="text-red-600 text-sm text-center">
                Health check failed. The system may still work — check server logs.
              </p>
            )}
            <button
              onClick={onComplete}
              className="w-full bg-th-input-bg border border-th-border text-th-text-secondary rounded px-4 py-2 text-sm font-medium hover:bg-th-surface-muted"
            >
              Continue to Login
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
