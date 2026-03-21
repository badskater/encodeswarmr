import { useState, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import * as api from '../api/client'
import type { Source, Job } from '../types'

type AudioFormat = 'FLAC' | 'Opus' | 'AAC'

const AUDIO_FORMATS: AudioFormat[] = ['FLAC', 'Opus', 'AAC']

function fmtBytes(n: number) {
  if (n >= 1_073_741_824) return (n / 1_073_741_824).toFixed(1) + ' GB'
  if (n >= 1_048_576) return (n / 1_048_576).toFixed(1) + ' MB'
  return (n / 1024).toFixed(1) + ' KB'
}

interface RowState {
  format: AudioFormat
  outputRoot: string
  submitting: boolean
  result: Job | null
  rowError: string
}

export default function AudioConvert() {
  const [sources, setSources] = useState<Source[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [rows, setRows] = useState<Record<string, RowState>>({})

  const load = useCallback(async () => {
    try {
      const s = await api.listSources()
      setSources(s)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load sources')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const getRow = (id: string): RowState =>
    rows[id] ?? { format: 'FLAC', outputRoot: '', submitting: false, result: null, rowError: '' }

  const setRow = (id: string, patch: Partial<RowState>) =>
    setRows(r => ({ ...r, [id]: { ...getRow(id), ...patch } }))

  const handleConvert = async (s: Source) => {
    const row = getRow(s.id)
    setRow(s.id, { submitting: true, result: null, rowError: '' })
    try {
      const job = await api.createJob({
        source_id: s.id,
        job_type: 'audio',
        encode_config: {
          ...(row.outputRoot ? { output_root: row.outputRoot } : {}),
          extra_vars: { AUDIO_FORMAT: row.format },
        },
      })
      setRow(s.id, { submitting: false, result: job })
    } catch (e: unknown) {
      setRow(s.id, { submitting: false, rowError: e instanceof Error ? e.message : 'Failed to create job' })
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold text-th-text">Audio Conversion</h1>
      {error && <p className="text-red-600 text-sm">{error}</p>}

      <div className="bg-th-surface rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-th-border text-sm">
          <thead className="bg-th-surface-muted">
            <tr>
              {['Filename', 'Path', 'Size', 'State', 'Format', 'Output Root (optional)', ''].map(h => (
                <th key={h} className="px-4 py-2 text-left text-xs font-medium text-th-text-muted uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-th-border-subtle">
            {sources.map(s => {
              const row = getRow(s.id)
              return (
                <tr key={s.id} className="hover:bg-th-surface-muted">
                  <td className="px-4 py-2 font-medium text-th-text">{s.filename}</td>
                  <td className="px-4 py-2 font-mono text-xs text-th-text-secondary max-w-xs truncate" title={s.path}>{s.path}</td>
                  <td className="px-4 py-2 text-th-text-secondary whitespace-nowrap">{fmtBytes(s.size_bytes)}</td>
                  <td className="px-4 py-2 text-th-text-secondary">{s.state}</td>
                  <td className="px-4 py-2">
                    <select
                      value={row.format}
                      onChange={e => setRow(s.id, { format: e.target.value as AudioFormat })}
                      className="bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-sm text-th-text"
                    >
                      {AUDIO_FORMATS.map(f => (
                        <option key={f} value={f}>{f}</option>
                      ))}
                    </select>
                  </td>
                  <td className="px-4 py-2">
                    <input
                      type="text"
                      value={row.outputRoot}
                      onChange={e => setRow(s.id, { outputRoot: e.target.value })}
                      placeholder="\\server\output"
                      className="bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-xs font-mono text-th-text w-48"
                    />
                  </td>
                  <td className="px-4 py-2 space-y-1">
                    <button
                      onClick={() => handleConvert(s)}
                      disabled={row.submitting}
                      className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50 whitespace-nowrap"
                    >
                      {row.submitting ? 'Submitting…' : 'Convert'}
                    </button>
                    {row.rowError && (
                      <p className="text-red-600 text-xs">{row.rowError}</p>
                    )}
                    {row.result && (
                      <p className="text-green-700 text-xs">
                        Job created:{' '}
                        <Link to={`/jobs/${row.result.id}`} className="underline hover:text-green-900">
                          {row.result.id.slice(0, 8)}…
                        </Link>
                      </p>
                    )}
                  </td>
                </tr>
              )
            })}
            {sources.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-4 text-center text-th-text-subtle">No sources available</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
