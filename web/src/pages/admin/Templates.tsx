import { useState, useEffect, useCallback } from 'react'
import * as api from '../../api/client'
import type { Template, Source } from '../../types'

function fmtDate(s: string) {
  return new Date(s).toLocaleString()
}

// ---------------------------------------------------------------------------
// Starter templates
// ---------------------------------------------------------------------------

interface StarterTemplate {
  name: string
  type: Template['type']
  description: string
  badge: string
  content: string
}

const STARTER_TEMPLATES: StarterTemplate[] = [
  {
    name: 'x265 Quality Encode',
    type: 'bat',
    badge: 'x265',
    description: 'CRF 18, slow preset, 10-bit main10 profile. Good starting point for high-quality archival encodes.',
    content: `@echo off
"%X265%" --input "{{.SOURCE_PATH}}" --output "{{.OUTPUT_PATH}}" --preset slow --crf 18 --profile main10 --level-idc 5.1 --aq-mode 3 --psy-rd 2.0 --psy-rdoq 1.0 --ref 6 --bframes 8 --rc-lookahead 60`,
  },
  {
    name: 'x265 HDR10 Passthrough',
    type: 'bat',
    badge: 'HDR10',
    description: 'Preserves HDR10 colour metadata using {{.HDR_TYPE}} conditional. Adds mastering display flags when source is HDR10.',
    content: `@echo off
set OPTS=--preset slow --crf 18 --profile main10 --level-idc 5.1
{{if eq .HDR_TYPE "hdr10"}}set OPTS=%OPTS% --hdr10 --hdr10-opt --colorprim bt2020 --transfer smpte2084 --colormatrix bt2020nc{{end}}
"%X265%" --input "{{.SOURCE_PATH}}" --output "{{.OUTPUT_PATH}}" %OPTS%`,
  },
  {
    name: 'x265 Dolby Vision',
    type: 'bat',
    badge: 'DV',
    description: 'Dolby Vision encode with profile-conditional flags using {{.DV_PROFILE}}. Requires RPU metadata file.',
    content: `@echo off
set OPTS=--preset slow --crf 18 --profile main10 --level-idc 5.1
{{if gt .DV_PROFILE 0}}set OPTS=%OPTS% {{dvFlag .DV_PROFILE}}{{end}}
"%X265%" --input "{{.SOURCE_PATH}}" --output "{{.OUTPUT_PATH}}" %OPTS%`,
  },
  {
    name: 'x264 Web Compatible',
    type: 'bat',
    badge: 'x264',
    description: 'CRF 23, medium preset, High 4.1 profile. Maximises browser and device compatibility for web delivery.',
    content: `@echo off
"%X264%" --input "{{.SOURCE_PATH}}" --output "{{.OUTPUT_PATH}}" --preset medium --crf 23 --profile high --level 4.1 --tune film --bframes 3 --ref 4 --me umh --subme 7 --trellis 1`,
  },
  {
    name: 'SVT-AV1 Encode',
    type: 'bat',
    badge: 'AV1',
    description: 'SVT-AV1 with preset 6 and CRF 30. Balanced quality/speed for AV1 delivery.',
    content: `@echo off
"%SVTAV1%" -i "{{.SOURCE_PATH}}" -b "{{.OUTPUT_PATH}}" --preset 6 --crf 30 --film-grain 0`,
  },
  {
    name: 'FFmpeg Stream Copy',
    type: 'bat',
    badge: 'ffmpeg',
    description: 'Remux without re-encoding. Copies all streams as-is into the output container.',
    content: `@echo off
"%FFMPEG%" -i "{{.SOURCE_PATH}}" -c copy "{{.OUTPUT_PATH}}"`,
  },
  {
    name: 'AviSynth Source',
    type: 'avs',
    badge: 'AVS',
    description: 'FFVideoSource frameserver with Trim for chunked encoding. Used as the frameserver template alongside a .bat encode script.',
    content: `FFVideoSource("{{escapeBat .SOURCE_PATH}}")
{{trimAvs .START_FRAME .END_FRAME}}`,
  },
  {
    name: 'VapourSynth Source',
    type: 'vpy',
    badge: 'VPY',
    description: 'ffms2 frameserver with trim for chunked encoding. Used as the frameserver template alongside a .bat encode script.',
    content: `import vapoursynth as vs
core = vs.core
clip = core.ffms2.Source("{{.SOURCE_PATH}}")
{{trimVpy .START_FRAME .END_FRAME}}
clip.set_output()`,
  },
]

// ---------------------------------------------------------------------------
// Variable reference data
// ---------------------------------------------------------------------------

const VAR_REF = {
  builtin: [
    { name: '{{.SOURCE_PATH}}', desc: 'Full UNC path to the source file' },
    { name: '{{.OUTPUT_PATH}}', desc: 'Full path for encoded output' },
    { name: '{{.START_FRAME}}', desc: 'First frame of this chunk (0-based)' },
    { name: '{{.END_FRAME}}', desc: 'Last frame of this chunk (inclusive)' },
    { name: '{{.CHUNK_INDEX}}', desc: 'Zero-based index of this chunk' },
    { name: '{{.TOTAL_CHUNKS}}', desc: 'Total number of chunks in the job' },
    { name: '{{.JOB_ID}}', desc: 'UUID of the parent job' },
    { name: '{{.TASK_ID}}', desc: 'UUID of this task' },
  ],
  hdr: [
    { name: '{{.HDR_TYPE}}', desc: 'hdr10 | hdr10plus | dolby_vision | hlg | (empty for SDR)' },
    { name: '{{.DV_PROFILE}}', desc: 'Dolby Vision profile number; 0 if not DV' },
  ],
  x265: [
    { name: '--preset', desc: 'ultrafast · superfast · veryfast · faster · fast · medium · slow · slower · veryslow · placebo' },
    { name: '--crf', desc: '0 (lossless) – 51 (worst); typical 18–22' },
    { name: '--profile', desc: 'main | main10 | main-still-picture' },
    { name: '--level-idc', desc: '1.0 – 6.2 (e.g. 5.1 for 4K)' },
    { name: '--tune', desc: 'psnr | ssim | grain | animation | zerolatency' },
    { name: '--colorprim / --transfer / --colormatrix', desc: 'HDR colour metadata (bt2020, smpte2084, bt2020nc)' },
    { name: '--max-cll / --master-display', desc: 'HDR10 static metadata strings' },
    { name: '--hdr10 / --hdr10-opt', desc: 'Enable HDR10 signalling and optimisation' },
    { name: '--dolby-vision-profile / --dolby-vision-rpu', desc: 'DV profile number and RPU file path' },
    { name: '--aq-mode', desc: '0–4 adaptive quantisation modes' },
    { name: '--psy-rd / --psy-rdoq', desc: 'Psycho-visual RD / RDOQ strength' },
    { name: '--ref', desc: 'Reference frames 1–16' },
    { name: '--bframes', desc: 'Max consecutive B-frames' },
    { name: '--rc-lookahead', desc: 'Frames to look ahead for rate control' },
    { name: '--deblock', desc: 'Deblocking filter (alpha:beta)' },
  ],
  x264: [
    { name: '--preset', desc: 'ultrafast · superfast · veryfast · faster · fast · medium · slow · slower · veryslow · placebo' },
    { name: '--crf', desc: '0 – 51; typical 18–23' },
    { name: '--profile', desc: 'baseline | main | high | high10 | high422 | high444' },
    { name: '--level', desc: '1.0 – 6.2' },
    { name: '--tune', desc: 'film | animation | grain | stillimage | psnr | ssim | fastdecode | zerolatency' },
    { name: '--me', desc: 'Motion estimation: dia | hex | umh | esa | tesa' },
    { name: '--subme', desc: 'Subpixel ME quality 0–11' },
    { name: '--trellis', desc: 'Trellis quantisation 0–2' },
    { name: '--ref', desc: 'Reference frames 1–16' },
    { name: '--bframes', desc: 'Max B-frames 0–16' },
    { name: '--b-adapt', desc: 'B-frame decision mode 0–2' },
    { name: '--aq-mode', desc: '0–3 adaptive quantisation' },
    { name: '--psy-rd', desc: 'Psycho-visual RD strength' },
    { name: '--deblock', desc: 'Deblocking filter (alpha:beta)' },
    { name: '--rc-lookahead', desc: 'Lookahead frames' },
  ],
  functions: [
    { name: '{{escapeBat .VAR}}', desc: 'Escape Windows batch special characters in a variable value' },
    { name: '{{basename .SOURCE_PATH}}', desc: 'Extract filename (without directory) from a path' },
    { name: '{{default "mkv" .EXT}}', desc: 'Return .EXT or "mkv" when .EXT is empty' },
    { name: '{{trimAvs .START_FRAME .END_FRAME}}', desc: 'Emit AviSynth Trim(start,end) or nothing for full source' },
    { name: '{{trimVpy .START_FRAME .END_FRAME}}', desc: 'Emit VapourSynth trim statement or nothing for full source' },
    { name: '{{gpuFlag .GPU_VENDOR}}', desc: 'GPU acceleration flag for the detected vendor' },
    { name: '{{dvFlag .DV_PROFILE}}', desc: 'Dolby Vision encoding flags for the given profile' },
    { name: '{{hdrFlag .HDR_TYPE}}', desc: 'HDR mastering display flags for the given HDR type' },
  ],
}

// ---------------------------------------------------------------------------
// VariableReference — collapsible reference panel
// ---------------------------------------------------------------------------

function VariableReference() {
  const [open, setOpen] = useState(false)

  const Section = ({ title, rows }: { title: string; rows: { name: string; desc: string }[] }) => (
    <div className="mb-3">
      <p className="text-xs font-semibold text-th-text-secondary mb-1">{title}</p>
      <table className="w-full text-xs border-collapse">
        <tbody>
          {rows.map(r => (
            <tr key={r.name} className="border-b border-th-border-subtle last:border-0">
              <td className="pr-3 py-0.5 align-top w-1/3 shrink-0">
                <code className="font-mono text-[0.7rem] bg-th-log-bg text-th-log-text px-1 py-0.5 rounded break-all">{r.name}</code>
              </td>
              <td className="py-0.5 align-top text-th-text-muted">{r.desc}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )

  return (
    <div className="border border-th-border-subtle rounded mt-2">
      <button
        type="button"
        onClick={() => setOpen(o => !o)}
        className="w-full flex items-center justify-between px-3 py-2 text-xs font-medium text-th-text-secondary hover:bg-th-surface-muted rounded"
      >
        <span>Variable Reference</span>
        <span className="text-th-text-subtle">{open ? '▲' : '▼'}</span>
      </button>
      {open && (
        <div className="px-3 pb-3 pt-1 space-y-1 overflow-auto max-h-80">
          <Section title="Built-in Variables" rows={VAR_REF.builtin} />
          <Section title="HDR / Dolby Vision" rows={VAR_REF.hdr} />
          <Section title="x265 Common Options" rows={VAR_REF.x265} />
          <Section title="x264 Common Options" rows={VAR_REF.x264} />
          <Section title="Template Functions" rows={VAR_REF.functions} />
          <p className="text-xs text-th-text-subtle mt-1">
            Global variables from the Variables page are also available as <code className="font-mono">{'{{.VAR_NAME}}'}</code>
          </p>
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// PreviewModal — renders a template dry-run and shows the output
// ---------------------------------------------------------------------------

interface PreviewModalProps {
  template: Template
  onClose: () => void
}

function PreviewModal({ template, onClose }: PreviewModalProps) {
  const [sources, setSources] = useState<Source[]>([])
  const [selectedSourceId, setSelectedSourceId] = useState('')
  const [variables, setVariables] = useState('')
  const [result, setResult] = useState<{ content: string; error?: string } | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    api.listSources().then(setSources).catch(() => {})
  }, [])

  const handlePreview = async () => {
    setLoading(true)
    setResult(null)
    try {
      let vars: Record<string, string> = {}
      if (variables.trim()) {
        try {
          vars = JSON.parse(variables)
        } catch {
          setResult({ content: '', error: 'Variables must be valid JSON, e.g. {"KEY": "value"}' })
          setLoading(false)
          return
        }
      }
      const resp = await api.previewTemplate(template.id, selectedSourceId || undefined, vars)
      setResult({ content: resp.content })
    } catch (e: unknown) {
      setResult({ content: '', error: e instanceof Error ? e.message : 'Preview failed' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-th-surface rounded-lg shadow-xl w-full max-w-3xl mx-4 flex flex-col max-h-[90vh]">
        <div className="flex items-center justify-between px-4 py-3 border-b border-th-border">
          <h2 className="text-sm font-semibold text-th-text">
            Preview: <span className="font-mono text-th-text-muted">{template.name}</span>
          </h2>
          <button type="button" onClick={onClose} className="text-xs text-th-text-muted hover:text-th-text">✕ Close</button>
        </div>

        <div className="p-4 space-y-3 flex-1 overflow-auto">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Source (optional)</label>
              <select
                value={selectedSourceId}
                onChange={e => setSelectedSourceId(e.target.value)}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
              >
                <option value="">— use placeholder path —</option>
                {sources.map(s => (
                  <option key={s.id} value={s.id}>{s.filename}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Extra Variables (JSON)</label>
              <input
                value={variables}
                onChange={e => setVariables(e.target.value)}
                placeholder='{"MY_VAR": "value"}'
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm font-mono text-th-text"
              />
            </div>
          </div>

          <button
            type="button"
            onClick={handlePreview}
            disabled={loading}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
          >
            {loading ? 'Rendering…' : 'Render Preview'}
          </button>

          {result && (
            <div className="space-y-1">
              {result.error ? (
                <div className="bg-red-50 dark:bg-red-950 border border-red-200 dark:border-red-800 rounded p-3">
                  <p className="text-xs font-semibold text-red-700 dark:text-red-300 mb-1">Template error</p>
                  <pre className="text-xs font-mono text-red-600 dark:text-red-400 whitespace-pre-wrap">{result.error}</pre>
                </div>
              ) : (
                <>
                  <p className="text-xs text-th-text-muted">Rendered output (.{template.extension})</p>
                  <pre className="bg-th-log-bg rounded p-3 text-xs font-mono text-th-log-text overflow-auto max-h-96 whitespace-pre">{result.content}</pre>
                </>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// StarterGrid — modal/panel showing starter template cards
// ---------------------------------------------------------------------------

interface StarterGridProps {
  onSelect: (s: StarterTemplate) => void
  onClose: () => void
}

function StarterGrid({ onSelect, onClose }: StarterGridProps) {
  const badgeColor: Record<string, string> = {
    x265: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
    HDR10: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
    DV: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
    x264: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
    AV1: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
    ffmpeg: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
    AVS: 'bg-teal-100 text-teal-800 dark:bg-teal-900 dark:text-teal-200',
    VPY: 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200',
  }

  return (
    <div className="bg-th-surface rounded-lg shadow p-4 space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-th-text-secondary">Choose a Starter Template</h2>
        <button type="button" onClick={onClose} className="text-xs text-th-text-muted hover:text-th-text">✕ Close</button>
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
        {STARTER_TEMPLATES.map(s => (
          <div key={s.name} className="border border-th-border-subtle rounded p-3 flex flex-col gap-1.5 hover:bg-th-surface-muted">
            <div className="flex items-center gap-2">
              <span className={`text-xs font-semibold px-1.5 py-0.5 rounded ${badgeColor[s.badge] ?? 'bg-th-surface-muted text-th-text-muted'}`}>
                {s.badge}
              </span>
              <span className="text-sm font-medium text-th-text">{s.name}</span>
              <span className="ml-auto font-mono text-xs text-th-text-subtle">.{s.type}</span>
            </div>
            <p className="text-xs text-th-text-muted leading-snug">{s.description}</p>
            <button
              type="button"
              onClick={() => onSelect(s)}
              className="self-start text-xs px-2 py-1 rounded bg-blue-600 text-white hover:bg-blue-700 mt-0.5"
            >
              Use
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------

export default function Templates() {
  const [templates, setTemplates] = useState<Template[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showForm, setShowForm] = useState(false)
  const [showStarters, setShowStarters] = useState(false)
  const [form, setForm] = useState({ name: '', type: 'bat' as Template['type'], description: '', content: '' })
  const [saving, setSaving] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [editValues, setEditValues] = useState<{ name: string; description: string; content: string }>({ name: '', description: '', content: '' })
  const [editSaving, setEditSaving] = useState(false)
  const [previewTemplate, setPreviewTemplate] = useState<Template | null>(null)

  const load = useCallback(async () => {
    try {
      const t = await api.listTemplates()
      setTemplates(t)
      setError('')
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to load')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    try {
      await api.createTemplate(form)
      setShowForm(false)
      setShowStarters(false)
      setForm({ name: '', type: 'bat', description: '', content: '' })
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create')
    } finally {
      setSaving(false)
    }
  }

  const handleSelectStarter = (s: StarterTemplate) => {
    setForm({ name: s.name, type: s.type, description: s.description, content: s.content })
    setShowStarters(false)
    setShowForm(true)
  }

  const startEdit = (t: Template) => {
    setEditId(t.id)
    setEditValues({ name: t.name, description: t.description ?? '', content: t.content })
  }

  const handleSaveEdit = async (id: string) => {
    setEditSaving(true)
    try {
      await api.updateTemplate(id, editValues)
      setEditId(null)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to save')
    } finally {
      setEditSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this template?')) return
    try {
      await api.deleteTemplate(id)
      load()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to delete')
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-th-text">Templates</h1>
        <div className="flex items-center gap-2">
          <button
            onClick={() => { setShowStarters(!showStarters); setShowForm(false) }}
            className="bg-th-surface-muted border border-th-border text-th-text-secondary px-3 py-1.5 rounded text-sm font-medium hover:bg-th-surface"
          >
            {showStarters ? 'Cancel' : 'Create from Starter'}
          </button>
          <button
            onClick={() => { setShowForm(!showForm); setShowStarters(false) }}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700"
          >
            {showForm ? 'Cancel' : 'Add Template'}
          </button>
        </div>
      </div>

      {error && <p className="text-red-600 text-sm">{error}</p>}

      {showStarters && (
        <StarterGrid
          onSelect={handleSelectStarter}
          onClose={() => setShowStarters(false)}
        />
      )}

      {showForm && (
        <form onSubmit={handleCreate} className="bg-th-surface rounded-lg shadow p-4 space-y-3">
          <h2 className="text-sm font-semibold text-th-text-secondary">New Template</h2>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Name</label>
              <input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" required />
            </div>
            <div>
              <label className="block text-xs text-th-text-muted mb-1">Type</label>
              <select value={form.type} onChange={e => setForm(f => ({ ...f, type: e.target.value as Template['type'] }))}
                className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text">
                <option value="bat">.bat</option>
                <option value="avs">.avs</option>
                <option value="vpy">.vpy</option>
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Description</label>
            <input value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text" />
          </div>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">Content</label>
            <textarea value={form.content} onChange={e => setForm(f => ({ ...f, content: e.target.value }))}
              rows={8}
              className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm font-mono text-th-text" required />
            <VariableReference />
          </div>
          <button type="submit" disabled={saving}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50">
            {saving ? 'Saving…' : 'Create Template'}
          </button>
        </form>
      )}

      {previewTemplate && (
        <PreviewModal template={previewTemplate} onClose={() => setPreviewTemplate(null)} />
      )}

      <div className="space-y-3">
        {templates.length === 0 && !loading && (
          <p className="text-th-text-subtle text-sm text-center py-4">No templates</p>
        )}
        {templates.map(t => (
          <div key={t.id} className="bg-th-surface rounded-lg shadow p-4">
            {editId === t.id ? (
              <div className="space-y-3">
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs text-th-text-muted mb-1">Name</label>
                    <input
                      value={editValues.name}
                      onChange={e => setEditValues(v => ({ ...v, name: e.target.value }))}
                      className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-th-text-muted mb-1">Description</label>
                    <input
                      value={editValues.description}
                      onChange={e => setEditValues(v => ({ ...v, description: e.target.value }))}
                      className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text"
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-xs text-th-text-muted mb-1">Content</label>
                  <textarea
                    value={editValues.content}
                    onChange={e => setEditValues(v => ({ ...v, content: e.target.value }))}
                    rows={12}
                    className="w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm font-mono text-th-text"
                  />
                  <VariableReference />
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => handleSaveEdit(t.id)}
                    disabled={editSaving}
                    className="bg-blue-600 text-white px-3 py-1.5 rounded text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
                  >
                    {editSaving ? 'Saving…' : 'Save'}
                  </button>
                  <button
                    onClick={() => setEditId(null)}
                    className="px-3 py-1.5 rounded text-sm text-th-text-muted border border-th-border hover:bg-th-surface-muted"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            ) : (
              <div>
                <div className="flex items-start justify-between mb-2">
                  <div>
                    <span className="font-medium text-th-text">{t.name}</span>
                    <span className="ml-2 font-mono text-xs bg-th-surface-muted px-1.5 py-0.5 rounded text-th-text-muted">.{t.type}</span>
                    {t.description && <span className="ml-2 text-sm text-th-text-muted">{t.description}</span>}
                  </div>
                  <div className="flex items-center gap-3 shrink-0">
                    <span className="text-xs text-th-text-subtle">{fmtDate(t.created_at)}</span>
                    <button onClick={() => setPreviewTemplate(t)} className="text-xs text-green-600 hover:underline">Preview</button>
                    <button onClick={() => startEdit(t)} className="text-xs text-blue-600 hover:underline">Edit</button>
                    <button onClick={() => handleDelete(t.id)} className="text-xs text-red-600 hover:underline">Delete</button>
                  </div>
                </div>
                <pre className="bg-th-log-bg rounded p-3 text-xs font-mono text-th-log-text overflow-auto max-h-40">{t.content}</pre>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
