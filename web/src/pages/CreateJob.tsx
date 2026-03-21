import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import type { Source, Template } from '../types'
import ChunkBoundaryPreview from '../components/ChunkBoundaryPreview'

// A JobPreset bundles commonly-used field combinations under a display name.
// These live in the frontend only; the backend deals with individual template IDs.
interface JobPreset {
  id: string
  name: string
  jobType: string
  runTemplateId: string
  fsTemplateId: string
  outputExt: string
  targetTags: string
  enableChunking: boolean
  chunkSizeFrames: number
  overlapFrames: number
}

function buildPresets(templates: Template[]): JobPreset[] {
  // Derive suggested presets from the available templates so the user
  // can quickly apply a commonly-used combination of run + frameserver templates.
  const batTemplates = templates.filter(t => t.type === 'bat')
  const fsTemplates = templates.filter(t => t.type === 'avs' || t.type === 'vpy')

  const presets: JobPreset[] = []

  // Pair each bat template with each frameserver template to create presets.
  batTemplates.forEach(bat => {
    fsTemplates.forEach(fs => {
      presets.push({
        id: `${bat.id}-${fs.id}`,
        name: `${bat.name} + ${fs.name} (.${fs.type})`,
        jobType: 'encode',
        runTemplateId: bat.id,
        fsTemplateId: fs.id,
        outputExt: 'mkv',
        targetTags: '',
        enableChunking: false,
        chunkSizeFrames: 1000,
        overlapFrames: 0,
      })
    })
    // Also offer a bat-only preset (no frameserver)
    if (fsTemplates.length === 0) {
      presets.push({
        id: bat.id,
        name: bat.name,
        jobType: 'encode',
        runTemplateId: bat.id,
        fsTemplateId: '',
        outputExt: 'mkv',
        targetTags: '',
        enableChunking: false,
        chunkSizeFrames: 1000,
        overlapFrames: 0,
      })
    }
  })

  return presets
}

export default function CreateJob() {
  const navigate = useNavigate()
  const [sources, setSources] = useState<Source[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [sceneLoading, setSceneLoading] = useState(false)
  const [sceneMessage, setSceneMessage] = useState('')

  // Template preset selection
  const [selectedPresetId, setSelectedPresetId] = useState('')

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

  // Chunked encoding config
  const [enableChunking, setEnableChunking] = useState(false)
  const [chunkSizeFrames, setChunkSizeFrames] = useState(1000)
  const [overlapFrames, setOverlapFrames] = useState(0)

  useEffect(() => {
    Promise.all([api.listSources(), api.listTemplates()])
      .then(([s, t]) => {
        setSources(s)
        setTemplates(t)
      })
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false))
  }, [])

  const presets = buildPresets(templates)

  const handleLoadPreset = (presetId: string) => {
    setSelectedPresetId(presetId)
    if (!presetId) return
    const preset = presets.find(p => p.id === presetId)
    if (!preset) return
    setJobType(preset.jobType)
    setRunTemplateId(preset.runTemplateId)
    setFsTemplateId(preset.fsTemplateId)
    setOutputExt(preset.outputExt)
    setTargetTags(preset.targetTags)
    setEnableChunking(preset.enableChunking)
    setChunkSizeFrames(preset.chunkSizeFrames)
    setOverlapFrames(preset.overlapFrames)
  }

  const handleLoadScenes = async () => {
    if (!selectedSource) return
    setSceneLoading(true)
    setSceneMessage('')
    try {
      const results = await api.listAnalysisResults(selectedSource.id)
      const sceneResult = results.find(r => r.type === 'scene')
      if (!sceneResult || !sceneResult.frame_data || sceneResult.frame_data.length === 0) {
        setSceneMessage('No scene analysis available — run an analysis job first')
        return
      }

      const summary = sceneResult.summary
      const fps =
        summary?.frame_count != null && summary?.duration_sec != null && summary.duration_sec > 0
          ? summary.frame_count / summary.duration_sec
          : 24
      const totalFrames = summary?.frame_count != null ? summary.frame_count : null

      const sceneFrames = sceneResult.frame_data
        .map(p => {
          if (p.frame != null) return p.frame
          if (p.pts != null) return Math.round(p.pts * fps)
          return null
        })
        .filter((f): f is number => f != null)
        .sort((a, b) => a - b)

      const lastFrame = totalFrames != null ? totalFrames - 1 : 999999
      const lines: string[] = []

      if (sceneFrames.length === 0) {
        setSceneMessage('No scene analysis available — run an analysis job first')
        return
      }

      lines.push(`0,${sceneFrames[0] - 1}`)
      for (let i = 0; i < sceneFrames.length - 1; i++) {
        lines.push(`${sceneFrames[i]},${sceneFrames[i + 1] - 1}`)
      }
      lines.push(`${sceneFrames[sceneFrames.length - 1]},${lastFrame}`)

      setChunksText(lines.join('\n'))
      setSceneMessage(`Loaded ${lines.length} scene boundaries from analysis`)
    } catch (e: unknown) {
      setSceneMessage(e instanceof Error ? e.message : 'Failed to load scene data')
    } finally {
      setSceneLoading(false)
    }
  }

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
          chunking_config: jobType === 'encode' && enableChunking
            ? { enable_chunking: true, chunk_size_frames: chunkSizeFrames, overlap_frames: overlapFrames }
            : undefined,
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
        {/* Load from template preset */}
        {presets.length > 0 && (
          <div className="rounded border border-th-border-subtle bg-th-surface-muted px-3 py-2 space-y-1">
            <label className="block text-xs font-medium text-th-text-secondary">Load from Template</label>
            <div className="flex items-center gap-2">
              <select
                value={selectedPresetId}
                onChange={e => handleLoadPreset(e.target.value)}
                className="flex-1 bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
              >
                <option value="">Select a preset…</option>
                {presets.map(p => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
              {selectedPresetId && (
                <button
                  type="button"
                  onClick={() => {
                    setSelectedPresetId('')
                    setRunTemplateId('')
                    setFsTemplateId('')
                    setOutputExt('mkv')
                    setTargetTags('')
                    setEnableChunking(false)
                  }}
                  className="text-xs px-2 py-1.5 rounded border border-th-input-border text-th-text-muted hover:bg-th-surface"
                >
                  Clear
                </button>
              )}
            </div>
            {selectedPresetId && (
              <p className="text-xs text-th-text-subtle">Form fields pre-filled from template — adjust as needed.</p>
            )}
          </div>
        )}

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

            {/* Chunked Encoding section */}
            <div className="rounded border border-th-border-subtle bg-th-surface-muted px-3 py-2 space-y-3">
              <div className="flex items-center gap-2">
                <input
                  id="enable-chunking"
                  type="checkbox"
                  checked={enableChunking}
                  onChange={e => setEnableChunking(e.target.checked)}
                  className="accent-blue-600"
                />
                <label htmlFor="enable-chunking" className="text-sm font-medium text-th-text select-none cursor-pointer">
                  Chunked Encoding
                </label>
              </div>

              {enableChunking && (
                <>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-xs text-th-text-muted mb-1">Chunk Size (frames)</label>
                      <input
                        type="number"
                        min={1}
                        value={chunkSizeFrames}
                        onChange={e => setChunkSizeFrames(Math.max(1, parseInt(e.target.value) || 1))}
                        className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-th-text-muted mb-1">Overlap (frames)</label>
                      <input
                        type="number"
                        min={0}
                        value={overlapFrames}
                        onChange={e => setOverlapFrames(Math.max(0, parseInt(e.target.value) || 0))}
                        className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                      />
                    </div>
                  </div>

                  {/* Scene boundary preview — fetches from API when a source is selected */}
                  {sourceId && (
                    <div>
                      <p className="text-xs text-th-text-muted mb-1">Scene boundary preview</p>
                      <ChunkBoundaryPreview sourceId={sourceId} />
                    </div>
                  )}
                </>
              )}
            </div>

            <div>
              <label className="block text-xs text-th-text-muted mb-1">Chunk Boundaries (one per line: start_frame,end_frame)</label>
              <textarea value={chunksText} onChange={e => setChunksText(e.target.value)}
                rows={5} placeholder={'0,1000\n1001,2000\n2001,3000'}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm font-mono text-th-text" />
              {selectedSource && (
                <div className="mt-1.5 flex items-center gap-2">
                  <button
                    type="button"
                    onClick={handleLoadScenes}
                    disabled={sceneLoading}
                    className="text-xs px-2 py-1 rounded border border-th-input-border bg-th-surface-muted text-th-text-secondary hover:bg-th-surface disabled:opacity-50"
                  >
                    {sceneLoading ? 'Loading scenes…' : 'Load Scene Boundaries'}
                  </button>
                  {sceneMessage && (
                    <span className="text-xs text-th-text-muted">{sceneMessage}</span>
                  )}
                </div>
              )}
              {chunksText.trim() && (
                <div className="mt-2">
                  <ChunkBoundaryPreview chunksText={chunksText} />
                </div>
              )}
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
