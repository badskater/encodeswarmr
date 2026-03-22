import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import type { Source } from '../types'
import type { ChainStep } from '../api/client'

// Audio codecs for the audio step config panel.
const AUDIO_CODECS = [
  { value: 'flac',       label: 'FLAC (lossless)' },
  { value: 'libopus',    label: 'Opus' },
  { value: 'libfdk_aac', label: 'AAC-LC (libfdk_aac)' },
  { value: 'aac',        label: 'AAC-LC (native)' },
  { value: 'ac3',        label: 'Dolby Digital (AC3)' },
  { value: 'eac3',       label: 'Dolby Digital Plus (E-AC3)' },
  { value: 'dca',        label: 'DTS' },
  { value: 'truehd',     label: 'Dolby TrueHD' },
  { value: 'pcm_s16le',  label: 'PCM 16-bit' },
  { value: 'pcm_s24le',  label: 'PCM 24-bit' },
  { value: 'libmp3lame', label: 'MP3' },
  { value: 'libvorbis',  label: 'Vorbis' },
]

// Default chain template: analysis → encode → audio → merge
const DEFAULT_STEPS: ChainStep[] = [
  { job_type: 'analysis',   name: 'HDR / Scene Detect' },
  { job_type: 'encode',     name: 'x265 Encode' },
  { job_type: 'audio',      name: 'Audio Extract', audio_config: { codec: 'flac' } },
  { job_type: 'merge',      name: 'Merge A/V' },
]

type WizardStep = 'source' | 'steps' | 'review'

export default function CreateJobChain() {
  const navigate = useNavigate()
  const [sources, setSources] = useState<Source[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  // Wizard state
  const [wizardStep, setWizardStep] = useState<WizardStep>('source')
  const [sourceId, setSourceId] = useState('')
  const [steps, setSteps] = useState<ChainStep[]>(DEFAULT_STEPS.map(s => ({ ...s })))

  const selectedSource = sources.find(s => s.id === sourceId) ?? null

  useEffect(() => {
    api.listSources()
      .then(setSources)
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load sources'))
      .finally(() => setLoading(false))
  }, [])

  const addStep = (jobType: string) => {
    const labels: Record<string, string> = {
      analysis: 'Analysis',
      encode: 'Encode',
      audio: 'Audio',
      merge: 'Merge A/V',
      hdr_detect: 'HDR Detect',
    }
    const newStep: ChainStep = {
      job_type: jobType,
      name: labels[jobType] ?? jobType,
      audio_config: jobType === 'audio' ? { codec: 'flac' } : undefined,
    }
    setSteps(prev => [...prev, newStep])
  }

  const removeStep = (idx: number) => {
    setSteps(prev => prev.filter((_, i) => i !== idx))
  }

  const updateStep = (idx: number, patch: Partial<ChainStep>) => {
    setSteps(prev => prev.map((s, i) => i === idx ? { ...s, ...patch } : s))
  }

  const updateAudioConfig = (idx: number, field: string, value: string | number) => {
    setSteps(prev => prev.map((s, i) => {
      if (i !== idx) return s
      return {
        ...s,
        audio_config: {
          ...s.audio_config,
          codec: s.audio_config?.codec ?? 'flac',
          [field]: value || undefined,
        },
      }
    }))
  }

  const handleSubmit = async () => {
    if (!sourceId) {
      setError('Please select a source')
      return
    }
    if (steps.length === 0) {
      setError('Please add at least one step')
      return
    }
    setSaving(true)
    setError('')
    try {
      const result = await api.createJobChain({ source_id: sourceId, steps })
      navigate(`/jobs?chain_group=${result.chain_group}`)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create job chain')
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="max-w-2xl space-y-4">
      <div className="flex items-center gap-3">
        <button onClick={() => navigate('/jobs')} className="text-blue-600 hover:underline text-sm">← Jobs</button>
        <h1 className="text-2xl font-bold text-th-text">New Job Chain</h1>
      </div>

      {/* Wizard progress bar */}
      <div className="flex gap-2 text-xs">
        {(['source', 'steps', 'review'] as WizardStep[]).map((s, i) => (
          <span key={s} className={`px-3 py-1 rounded-full ${wizardStep === s ? 'bg-blue-600 text-white' : 'bg-th-surface-muted text-th-text-muted'}`}>
            {i + 1}. {s.charAt(0).toUpperCase() + s.slice(1)}
          </span>
        ))}
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {/* Step 1: Select source */}
      {wizardStep === 'source' && (
        <div className="bg-th-surface rounded-lg shadow p-4 space-y-4">
          <h2 className="text-lg font-semibold text-th-text">1. Select Source</h2>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Source *</label>
            <select value={sourceId} onChange={e => setSourceId(e.target.value)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
              <option value="">Select a source…</option>
              {sources.map(s => (
                <option key={s.id} value={s.id}>{s.filename} ({s.path})</option>
              ))}
            </select>
          </div>
          {selectedSource && (
            <div className="rounded border border-th-border-subtle bg-th-surface-muted px-3 py-2 text-xs space-y-0.5">
              <p className="text-th-text-muted font-medium">Selected source</p>
              <p className="text-th-text-secondary">{selectedSource.filename}</p>
              <p className="text-th-text-muted">{selectedSource.path}</p>
            </div>
          )}
          <div className="flex justify-end">
            <button
              onClick={() => { if (sourceId) setWizardStep('steps'); else setError('Please select a source') }}
              className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700"
            >
              Next: Configure Steps →
            </button>
          </div>
        </div>
      )}

      {/* Step 2: Configure steps */}
      {wizardStep === 'steps' && (
        <div className="bg-th-surface rounded-lg shadow p-4 space-y-4">
          <h2 className="text-lg font-semibold text-th-text">2. Configure Steps</h2>
          <p className="text-xs text-th-text-muted">
            Jobs run sequentially — each waits for the previous one to complete.
          </p>

          <div className="space-y-3">
            {steps.map((step, idx) => (
              <div key={idx} className="rounded border border-th-border bg-th-surface-muted p-3 space-y-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-mono bg-th-log-bg text-th-log-text px-1.5 py-0.5 rounded">
                      {idx + 1}
                    </span>
                    <span className="text-sm font-medium text-th-text">{step.job_type}</span>
                  </div>
                  <button
                    onClick={() => removeStep(idx)}
                    className="text-xs text-red-500 hover:text-red-700"
                  >
                    Remove
                  </button>
                </div>
                <div>
                  <label className="block text-xs text-th-text-muted mb-0.5">Name</label>
                  <input
                    value={step.name ?? ''}
                    onChange={e => updateStep(idx, { name: e.target.value })}
                    className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-sm text-th-text"
                  />
                </div>
                {step.job_type === 'audio' && (
                  <div className="grid grid-cols-2 gap-2">
                    <div>
                      <label className="block text-xs text-th-text-muted mb-0.5">Codec</label>
                      <select
                        value={step.audio_config?.codec ?? 'flac'}
                        onChange={e => updateAudioConfig(idx, 'codec', e.target.value)}
                        className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-sm text-th-text"
                      >
                        {AUDIO_CODECS.map(c => (
                          <option key={c.value} value={c.value}>{c.label}</option>
                        ))}
                      </select>
                    </div>
                    <div>
                      <label className="block text-xs text-th-text-muted mb-0.5">Bitrate</label>
                      <input
                        value={step.audio_config?.bitrate ?? ''}
                        onChange={e => updateAudioConfig(idx, 'bitrate', e.target.value)}
                        placeholder="320k"
                        className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-sm text-th-text"
                      />
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>

          <div className="flex flex-wrap gap-2">
            {['analysis', 'encode', 'audio', 'merge', 'hdr_detect'].map(jt => (
              <button
                key={jt}
                type="button"
                onClick={() => addStep(jt)}
                className="text-xs px-2 py-1 rounded border border-th-input-border bg-th-surface-muted text-th-text-secondary hover:bg-th-surface"
              >
                + {jt}
              </button>
            ))}
          </div>

          <div className="flex justify-between">
            <button
              onClick={() => setWizardStep('source')}
              className="text-sm text-th-text-muted hover:text-th-text"
            >
              ← Back
            </button>
            <button
              onClick={() => { if (steps.length > 0) setWizardStep('review'); else setError('Add at least one step') }}
              className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700"
            >
              Next: Review →
            </button>
          </div>
        </div>
      )}

      {/* Step 3: Review & create */}
      {wizardStep === 'review' && (
        <div className="bg-th-surface rounded-lg shadow p-4 space-y-4">
          <h2 className="text-lg font-semibold text-th-text">3. Review Chain</h2>

          <div className="rounded border border-th-border-subtle bg-th-surface-muted px-3 py-2 text-sm space-y-1">
            <p><span className="text-th-text-muted">Source:</span> {selectedSource?.filename}</p>
            <p><span className="text-th-text-muted">Steps:</span> {steps.length}</p>
          </div>

          <div className="space-y-2">
            {steps.map((step, idx) => (
              <div key={idx} className="flex items-center gap-3 text-sm">
                <span className="text-xs font-mono bg-blue-100 text-blue-800 px-1.5 py-0.5 rounded dark:bg-blue-900 dark:text-blue-200">
                  {idx + 1}
                </span>
                <span className="font-medium text-th-text">{step.job_type}</span>
                {step.name && <span className="text-th-text-muted">— {step.name}</span>}
                {step.audio_config && (
                  <span className="text-xs text-th-text-subtle">({step.audio_config.codec})</span>
                )}
                {idx < steps.length - 1 && (
                  <span className="ml-auto text-th-text-muted text-xs">→ waits →</span>
                )}
              </div>
            ))}
          </div>

          <div className="flex justify-between">
            <button
              onClick={() => setWizardStep('steps')}
              className="text-sm text-th-text-muted hover:text-th-text"
            >
              ← Back
            </button>
            <button
              onClick={handleSubmit}
              disabled={saving}
              className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
            >
              {saving ? 'Creating…' : 'Create Job Chain'}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
