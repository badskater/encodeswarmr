import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import type { Source, Template, Job } from '../types'
import type { Source, Template, AudioPreset } from '../types'
import type { Flow } from '../types/flow'
import ChunkBoundaryPreview from '../components/ChunkBoundaryPreview'

// Supported audio codecs displayed in the audio job config panel.
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

export default function CreateJob() {
  const navigate = useNavigate()
  const [sources, setSources] = useState<Source[]>([])
  const [templates, setTemplates] = useState<Template[]>([])
  const [jobs, setJobs] = useState<Job[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [sceneLoading, setSceneLoading] = useState(false)
  const [sceneMessage, setSceneMessage] = useState('')

  const [flows, setFlows] = useState<Flow[]>([])
  const [useFlow, setUseFlow] = useState(false)
  const [selectedFlowId, setSelectedFlowId] = useState('')
  const [audioPresets, setAudioPresets] = useState<AudioPreset[]>([])
  const [audioPresetName, setAudioPresetName] = useState('')

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

  // Job chaining
  const [dependsOnJobId, setDependsOnJobId] = useState('')

  // Audio config (for audio job type)
  const [audioCodec, setAudioCodec] = useState('flac')
  const [audioBitrate, setAudioBitrate] = useState('')
  const [audioChannels, setAudioChannels] = useState(0)
  const [audioSampleRate, setAudioSampleRate] = useState(0)

  useEffect(() => {
    Promise.all([api.listSources(), api.listTemplates(), api.listFlows(), api.listJobs()])
      .then(([s, t, fl, j]) => {
        setSources(s)
        setTemplates(t)
        setFlows(fl)
        // Only show active jobs for "chain after" selection
        setJobs(j.filter((j: Job) => j.status !== 'completed' && j.status !== 'failed' && j.status !== 'cancelled'))
    Promise.all([api.listSources(), api.listTemplates(), api.listFlows(), api.listAudioPresets()])
      .then(([s, t, fl, ap]) => {
        setSources(s)
        setTemplates(t)
        setFlows(fl)
        setAudioPresets(ap)
      })
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false))
  }, [])

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
          run_script_template_id: (!useFlow && runTemplateId) ? runTemplateId : undefined,
          frameserver_template_id: (!useFlow && fsTemplateId) ? fsTemplateId : undefined,
          chunk_boundaries: chunkBoundaries,
          output_root: outputRoot || undefined,
          output_extension: outputExt || undefined,
          chunking_config: jobType === 'encode' && enableChunking
            ? { enable_chunking: true, chunk_size_frames: chunkSizeFrames, overlap_frames: overlapFrames }
            : undefined,
        },
        flow_id: (useFlow && selectedFlowId) ? selectedFlowId : undefined,
        audio_config: jobType === 'audio' ? {
          codec: audioCodec,
          bitrate: audioBitrate || undefined,
          channels: audioChannels || undefined,
          sample_rate: audioSampleRate || undefined,
        } : undefined,
        depends_on: dependsOnJobId || undefined,
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

  const selectedRunTemplate = templates.find(t => t.id === runTemplateId) ?? null
  const selectedFsTemplate = templates.find(t => t.id === fsTemplateId) ?? null

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

        {/* Use Flow toggle */}
        <div className="flex items-center gap-3 p-3 rounded-lg border border-th-border bg-th-surface-muted">
          <button
            type="button"
            role="switch"
            aria-checked={useFlow}
            onClick={() => setUseFlow(v => !v)}
            className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors focus:outline-none ${
              useFlow ? 'bg-blue-600' : 'bg-th-border'
            }`}
          >
            <span
              className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow transition-transform ${
                useFlow ? 'translate-x-4' : 'translate-x-0.5'
              }`}
            />
          </button>
          <div>
            <p className="text-sm font-medium text-th-text">Use Flow Pipeline</p>
            <p className="text-xs text-th-text-muted">Select a saved flow to drive this job instead of individual templates</p>
          </div>
          {flows.length > 0 && (
            <a href="/flows" className="ml-auto text-xs text-blue-500 hover:underline flex-shrink-0">
              Manage Flows →
            </a>
          )}
        </div>

        {useFlow && (
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Flow *</label>
            <select
              value={selectedFlowId}
              onChange={e => setSelectedFlowId(e.target.value)}
              required={useFlow}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
            >
              <option value="">Select a flow…</option>
              {flows.map(f => (
                <option key={f.id} value={f.id}>{f.name}{f.description ? ` — ${f.description}` : ''}</option>
              ))}
            </select>
            {flows.length === 0 && (
              <p className="text-xs text-th-text-subtle mt-1">
                No flows created yet.{' '}
                <a href="/flows/editor" className="text-blue-500 hover:underline">Create one</a>
              </p>
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
              <option value="merge">Merge A/V</option>
              <option value="hdr_detect">HDR Detect</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Priority</label>
            <input type="number" value={priority} onChange={e => setPriority(parseInt(e.target.value) || 0)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" />
          </div>
        </div>

        {/* Chain after another job */}
        {jobs.length > 0 && (
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Chain After Job (optional)</label>
            <select value={dependsOnJobId} onChange={e => setDependsOnJobId(e.target.value)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
              <option value="">— No dependency —</option>
              {jobs.map(j => (
                <option key={j.id} value={j.id}>
                  {j.job_type} · {j.status} · {j.id.slice(0, 8)}…
                </option>
              ))}
            </select>
            <p className="text-xs text-th-text-muted mt-0.5">
              This job will stay in <em>waiting</em> status until the selected job completes.
            </p>
          </div>
        )}

        {/* Audio codec config */}
        {jobType === 'audio' && (
          <div className="rounded border border-th-border-subtle bg-th-surface-muted px-3 py-3 space-y-3">
            <p className="text-sm font-medium text-th-text">Audio Configuration</p>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Codec *</label>
                <select value={audioCodec} onChange={e => setAudioCodec(e.target.value)}
                  className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
                  {AUDIO_CODECS.map(c => (
                    <option key={c.value} value={c.value}>{c.label}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Bitrate (lossy)</label>
                <input value={audioBitrate} onChange={e => setAudioBitrate(e.target.value)}
                  placeholder="320k"
                  className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" />
              </div>
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Channels</label>
                <select value={audioChannels} onChange={e => setAudioChannels(parseInt(e.target.value))}
                  className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
                  <option value={0}>Default</option>
                  <option value={2}>2 (stereo)</option>
                  <option value={6}>6 (5.1)</option>
                  <option value={8}>8 (7.1)</option>
                </select>
              </div>
              <div>
                <label className="block text-xs text-th-text-muted mb-1">Sample Rate (Hz)</label>
                <select value={audioSampleRate} onChange={e => setAudioSampleRate(parseInt(e.target.value))}
                  className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
                  <option value={0}>Default</option>
                  <option value={44100}>44100</option>
                  <option value={48000}>48000</option>
                  <option value={96000}>96000</option>
                </select>
              </div>
            </div>
        {/* Audio codec selector — shown for audio job type */}
        {jobType === 'audio' && audioPresets.length > 0 && (
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Audio Codec Preset</label>
            <select
              value={audioPresetName}
              onChange={e => setAudioPresetName(e.target.value)}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
            >
              <option value="">Select audio preset…</option>
              {audioPresets.map(ap => (
                <option key={ap.name} value={ap.name}>
                  {ap.name}{ap.bitrate ? ` (${ap.bitrate})` : ''}
                </option>
              ))}
            </select>
            {audioPresets.find(ap => ap.name === audioPresetName)?.description && (
              <p className="text-xs text-th-text-muted mt-0.5">
                {audioPresets.find(ap => ap.name === audioPresetName)!.description}
              </p>
            )}
          </div>
        )}

        <div>
          <label className="block text-xs text-th-text-muted mb-1">Target Tags (comma-separated)</label>
          <input value={targetTags} onChange={e => setTargetTags(e.target.value)} placeholder="gpu,nvenc"
            className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" />
        </div>

        {/* Run Script Template */}
        <div>
          <label className="block text-xs text-th-text-muted mb-1">Run Script Template</label>
          <select value={runTemplateId} onChange={e => setRunTemplateId(e.target.value)}
            className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
            <option value="">None</option>
            {batTemplates.map(t => (
              <option key={t.id} value={t.id}>{t.name}</option>
            ))}
          </select>
          {selectedRunTemplate?.description && (
            <span className="text-xs text-th-text-muted mt-0.5 block">{selectedRunTemplate.description}</span>
          )}
          {selectedRunTemplate && (
            <details className="mt-1.5">
              <summary className="text-xs text-th-text-subtle cursor-pointer select-none hover:text-th-text-muted">
                Preview template content
              </summary>
              <pre className="mt-1 bg-th-log-bg rounded p-2 text-xs font-mono text-th-log-text overflow-auto max-h-32">{selectedRunTemplate.content}</pre>
            </details>
          )}
        </div>

        {jobType === 'encode' && (
          <>
            {/* Frameserver Template */}
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Frameserver Template</label>
              <select value={fsTemplateId} onChange={e => setFsTemplateId(e.target.value)}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
                <option value="">None</option>
                {fsTemplates.map(t => (
                  <option key={t.id} value={t.id}>{t.name} (.{t.type})</option>
                ))}
              </select>
              {selectedFsTemplate?.description && (
                <span className="text-xs text-th-text-muted mt-0.5 block">{selectedFsTemplate.description}</span>
              )}
              {selectedFsTemplate && (
                <details className="mt-1.5">
                  <summary className="text-xs text-th-text-subtle cursor-pointer select-none hover:text-th-text-muted">
                    Preview template content
                  </summary>
                  <pre className="mt-1 bg-th-log-bg rounded p-2 text-xs font-mono text-th-log-text overflow-auto max-h-32">{selectedFsTemplate.content}</pre>
                </details>
              )}
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
              <p className="text-xs text-th-text-muted -mt-1">
                {enableChunking
                  ? 'Source split into chunks for parallel agent encoding.'
                  : 'Entire source encoded as a single task.'}
              </p>

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

                  {/* Scene boundary preview */}
                  {sourceId && (
                    <div>
                      <p className="text-xs text-th-text-muted mb-1">Scene boundary preview</p>
                      <ChunkBoundaryPreview sourceId={sourceId} />
                    </div>
                  )}

                  {/* Load Scene Boundaries — only shown when chunking is enabled */}
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
