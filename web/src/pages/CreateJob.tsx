import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import type { Source, Template } from '../types'

export default function CreateJob() {
  const navigate = useNavigate()
  const [sources, setSources] = useState<Source[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const [sourceId, setSourceId] = useState('')
  const [jobType, setJobType] = useState('encode')

  const selectedSource = sources.find(s => s.id === sourceId) ?? null
  const [priority, setPriority] = useState(0)
  const [targetTags, setTargetTags] = useState('')
  const [runTemplateId, setRunTemplateId] = useState('')
  const [fsTemplateId, setFsTemplateId] = useState('')
  const [outputRoot, setOutputRoot] = useState('')
  const [outputExt, setOutputExt] = useState('mkv')
  const [chunksText, setChunksText] = useState('')

  useEffect(() => {
    Promise.all([api.listSources(), api.listTemplates()])
      .then(([s, t]) => {
        setSources(s)
        setTemplates(t)
      })
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false))
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError('')

    try {
      let chunkBoundaries: { start_frame: number; end_frame: number }[] | undefined
      if (jobType === 'encode' && chunksText.trim()) {
        chunkBoundaries = chunksText.trim().split('\n').map(line => {
          const parts = line.split(',').map(s => parseInt(s.trim(), 10))
          if (parts.length !== 2 || isNaN(parts[0]) || isNaN(parts[1])) {
            throw new Error(`Invalid chunk boundary line: "${line}" — expected "start,end"`)
          }
          return { start_frame: parts[0], end_frame: parts[1] }
        })
      }

      const tags = targetTags.split(',').map(t => t.trim()).filter(Boolean)

      const job = await api.createJob({
        source_id: sourceId,
        job_type: jobType,
        priority,
        target_tags: tags,
        encode_config: {
          run_script_template_id: runTemplateId || undefined,
          frameserver_template_id: fsTemplateId || undefined,
          chunk_boundaries: chunkBoundaries,
          output_root: outputRoot || undefined,
          output_extension: outputExt || undefined,
        },
      })

      navigate(`/jobs/${job.id}`)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create job')
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  const batTemplates = templates.filter(t => t.type === 'bat')
  const fsTemplates = templates.filter(t => t.type === 'avs' || t.type === 'vpy')

  function hdrLabel(hdrType: string, dvProfile: number): string {
    if (hdrType === 'dolby_vision') return dvProfile > 0 ? `Dolby Vision Profile ${dvProfile}` : 'Dolby Vision'
    if (hdrType === 'hdr10plus') return 'HDR10+'
    if (hdrType === 'hdr10') return 'HDR10'
    if (hdrType === 'hlg') return 'HLG'
    return 'Unknown / SDR'
  }

  return (
    <div className="max-w-2xl space-y-4">
      <div className="flex items-center gap-3">
        <button onClick={() => navigate('/jobs')} className="text-blue-600 hover:underline text-sm">← Jobs</button>
        <h1 className="text-2xl font-bold text-th-text">New Job</h1>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      <form onSubmit={handleSubmit} className="bg-th-surface rounded-lg shadow p-4 space-y-4">
        <div>
          <label className="block text-xs text-th-text-muted mb-1">Source *</label>
          <select value={sourceId} onChange={e => setSourceId(e.target.value)} required
            className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
            <option value="">Select a source…</option>
            {sources.map(s => (
              <option key={s.id} value={s.id}>{s.filename} ({s.path})</option>
            ))}
          </select>
        </div>

        {selectedSource && (
          <div className="rounded border border-th-border-subtle bg-th-surface-muted px-3 py-2 text-xs space-y-0.5">
            <p className="text-th-text-muted font-medium mb-1">Source metadata</p>
            <p className="text-th-text-secondary">
              <span className="text-th-text-muted">HDR:</span>{' '}
              {hdrLabel(selectedSource.hdr_type, selectedSource.dv_profile)}
            </p>
            {selectedSource.dv_profile > 0 && (
              <p className="text-th-text-secondary">
                <span className="text-th-text-muted">DV Profile:</span> {selectedSource.dv_profile}
              </p>
            )}
            <p className="text-th-text-subtle mt-1">
              Use <code className="font-mono">{'{{.HDR_TYPE}}'}</code> and{' '}
              <code className="font-mono">{'{{.DV_PROFILE}}'}</code> in script templates.
            </p>
          </div>
        )}

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Job Type *</label>
            <select value={jobType} onChange={e => setJobType(e.target.value)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
              <option value="encode">Encode</option>
              <option value="analysis">Analysis</option>
              <option value="audio">Audio</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Priority</label>
            <input type="number" value={priority} onChange={e => setPriority(parseInt(e.target.value) || 0)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" />
          </div>
        </div>

        <div>
          <label className="block text-xs text-th-text-muted mb-1">Target Tags (comma-separated)</label>
          <input value={targetTags} onChange={e => setTargetTags(e.target.value)} placeholder="gpu,nvenc"
            className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" />
        </div>

        <div>
          <label className="block text-xs text-th-text-muted mb-1">Run Script Template</label>
          <select value={runTemplateId} onChange={e => setRunTemplateId(e.target.value)}
            className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
            <option value="">None</option>
            {batTemplates.map(t => (
              <option key={t.id} value={t.id}>{t.name}</option>
            ))}
          </select>
        </div>

        {jobType === 'encode' && (
          <>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Frameserver Template</label>
              <select value={fsTemplateId} onChange={e => setFsTemplateId(e.target.value)}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
                <option value="">None</option>
                {fsTemplates.map(t => (
                  <option key={t.id} value={t.id}>{t.name} (.{t.type})</option>
                ))}
              </select>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Output Root (UNC path)</label>
                <input value={outputRoot} onChange={e => setOutputRoot(e.target.value)} placeholder="\\server\share\output"
                  className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" />
              </div>
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Output Extension</label>
                <input value={outputExt} onChange={e => setOutputExt(e.target.value)} placeholder="mkv"
                  className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" />
              </div>
            </div>

            <div>
              <label className="block text-xs text-th-text-muted mb-1">Chunk Boundaries (one per line: start_frame,end_frame)</label>
              <textarea value={chunksText} onChange={e => setChunksText(e.target.value)}
                rows={5} placeholder={'0,1000\n1001,2000\n2001,3000'}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm font-mono text-th-text" />
            </div>
          </>
        )}

        <button type="submit" disabled={saving}
          className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
          {saving ? 'Creating…' : 'Create Job'}
        </button>
      </form>
    </div>
  )
}
