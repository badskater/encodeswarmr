import { useState, useEffect } from 'react'
import type { Node } from '@xyflow/react'
import type { FlowNodeData } from '../types/flow'
import { CATEGORY_COLORS, NODE_REGISTRY_MAP } from './nodeRegistry'
import * as api from '../api/client'
import type { Template, Webhook } from '../types'

interface Props {
  node: Node
  onUpdate: (nodeId: string, config: Record<string, unknown>) => void
  onClose: () => void
}

// ---------------------------------------------------------------------------
// Field renderers
// ---------------------------------------------------------------------------

const inputCls =
  'w-full bg-th-input-bg border border-th-input-border rounded px-2 py-1.5 text-sm text-th-text focus:outline-none focus:ring-1'

function TextField({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  placeholder?: string
}) {
  return (
    <div>
      <label className="block text-xs text-th-text-muted mb-1">{label}</label>
      <input
        type="text"
        className={inputCls}
        value={value}
        onChange={e => onChange(e.target.value)}
        placeholder={placeholder}
      />
    </div>
  )
}

function NumberField({
  label,
  value,
  onChange,
  min,
  max,
  step,
}: {
  label: string
  value: number
  onChange: (v: number) => void
  min?: number
  max?: number
  step?: number
}) {
  return (
    <div>
      <label className="block text-xs text-th-text-muted mb-1">{label}</label>
      <input
        type="number"
        className={inputCls}
        value={value}
        min={min}
        max={max}
        step={step ?? 1}
        onChange={e => onChange(Number(e.target.value))}
      />
    </div>
  )
}

function CRFSlider({
  value,
  onChange,
}: {
  value: number
  onChange: (v: number) => void
}) {
  return (
    <div>
      <label className="block text-xs text-th-text-muted mb-1">
        CRF — <span className="font-semibold text-th-text">{value}</span>{' '}
        <span className="opacity-60">(lower = better quality)</span>
      </label>
      <input
        type="range"
        min={0}
        max={51}
        value={value}
        onChange={e => onChange(Number(e.target.value))}
        className="w-full accent-blue-500"
      />
      <div className="flex justify-between text-xs text-th-text-subtle mt-0.5">
        <span>0 lossless</span>
        <span>51 worst</span>
      </div>
    </div>
  )
}

function SelectField({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: { value: string; label: string }[]
  onChange: (v: string) => void
}) {
  return (
    <div>
      <label className="block text-xs text-th-text-muted mb-1">{label}</label>
      <select
        className={inputCls}
        value={value}
        onChange={e => onChange(e.target.value)}
      >
        {options.map(o => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Per-node-type config forms
// ---------------------------------------------------------------------------

const X265_PRESETS = [
  'ultrafast', 'superfast', 'veryfast', 'faster', 'fast',
  'medium', 'slow', 'slower', 'veryslow', 'placebo',
]
const X264_PRESETS = [...X265_PRESETS]
const X265_PROFILES = ['main', 'main10', 'mainstillpicture', 'main444-8', 'main444-10']
const X264_PROFILES = ['baseline', 'main', 'high', 'high10', 'high422', 'high444']
const NVENC_PRESETS = ['p1', 'p2', 'p3', 'p4', 'p5', 'p6', 'p7']

const TWOPASS_CODEC_OPTIONS = [
  { value: 'x265', label: 'x265 (HEVC)' },
  { value: 'x264', label: 'x264 (H.264)' },
]

function ConfigFields({
  nodeType,
  config,
  onChange,
  templates,
  webhooks,
}: {
  nodeType: string
  config: Record<string, unknown>
  onChange: (c: Record<string, unknown>) => void
  templates: Template[]
  webhooks: Webhook[]
}) {
  const set = (key: string, val: unknown) => onChange({ ...config, [key]: val })

  switch (nodeType) {
    case 'encode_x265':
      return (
        <>
          <CRFSlider value={Number(config.crf ?? 18)} onChange={v => set('crf', v)} />
          <SelectField
            label="Preset"
            value={String(config.preset ?? 'slow')}
            options={X265_PRESETS.map(p => ({ value: p, label: p }))}
            onChange={v => set('preset', v)}
          />
          <SelectField
            label="Profile"
            value={String(config.profile ?? 'main10')}
            options={X265_PROFILES.map(p => ({ value: p, label: p }))}
            onChange={v => set('profile', v)}
          />
          <TextField
            label="Tune (optional)"
            value={String(config.tune ?? '')}
            onChange={v => set('tune', v)}
            placeholder="film, animation, grain…"
          />
          <TextField
            label="Extra args"
            value={String(config.extra_args ?? '')}
            onChange={v => set('extra_args', v)}
            placeholder="--aq-mode 3 --psy-rd 2.0"
          />
        </>
      )

    case 'encode_x264':
      return (
        <>
          <CRFSlider value={Number(config.crf ?? 23)} onChange={v => set('crf', v)} />
          <SelectField
            label="Preset"
            value={String(config.preset ?? 'medium')}
            options={X264_PRESETS.map(p => ({ value: p, label: p }))}
            onChange={v => set('preset', v)}
          />
          <SelectField
            label="Profile"
            value={String(config.profile ?? 'high')}
            options={X264_PROFILES.map(p => ({ value: p, label: p }))}
            onChange={v => set('profile', v)}
          />
          <TextField
            label="Tune (optional)"
            value={String(config.tune ?? '')}
            onChange={v => set('tune', v)}
            placeholder="film, animation, grain, zerolatency…"
          />
        </>
      )

    case 'encode_svtav1':
      return (
        <>
          <CRFSlider value={Number(config.crf ?? 30)} onChange={v => set('crf', v)} />
          <NumberField
            label="Preset (0 = slowest, 12 = fastest)"
            value={Number(config.preset ?? 6)}
            onChange={v => set('preset', v)}
            min={0}
            max={12}
          />
        </>
      )

    case 'encode_ffmpeg_copy':
      return (
        <SelectField
          label="Container"
          value={String(config.container ?? 'mkv')}
          options={[
            { value: 'mkv', label: 'Matroska (.mkv)' },
            { value: 'mp4', label: 'MP4 (.mp4)' },
            { value: 'ts', label: 'MPEG-TS (.ts)' },
          ]}
          onChange={v => set('container', v)}
        />
      )

    case 'encode_nvenc':
      return (
        <>
          <NumberField
            label="CQ (0-51, lower = better)"
            value={Number(config.cq ?? 28)}
            onChange={v => set('cq', v)}
            min={0}
            max={51}
          />
          <SelectField
            label="Preset"
            value={String(config.preset ?? 'p4')}
            options={NVENC_PRESETS.map(p => ({ value: p, label: p }))}
            onChange={v => set('preset', v)}
          />
        </>
      )

    case 'analyze_scene':
      return (
        <NumberField
          label="Scene threshold (0.1–1.0)"
          value={Number(config.threshold ?? 0.4)}
          onChange={v => set('threshold', v)}
          min={0.1}
          max={1.0}
          step={0.05}
        />
      )

    case 'cond_file_size':
      return (
        <>
          <SelectField
            label="Operator"
            value={String(config.operator ?? 'gt')}
            options={[
              { value: 'gt', label: 'Greater than (>)' },
              { value: 'lt', label: 'Less than (<)' },
              { value: 'eq', label: 'Equal to (=)' },
            ]}
            onChange={v => set('operator', v)}
          />
          <NumberField
            label="Size threshold (MB)"
            value={Number(config.size_mb ?? 1000)}
            onChange={v => set('size_mb', v)}
            min={0}
          />
        </>
      )

    case 'cond_resolution':
      return (
        <>
          <NumberField
            label="Min Width (px)"
            value={Number(config.min_width ?? 1920)}
            onChange={v => set('min_width', v)}
            min={1}
          />
          <NumberField
            label="Min Height (px)"
            value={Number(config.min_height ?? 1080)}
            onChange={v => set('min_height', v)}
            min={1}
          />
        </>
      )

    case 'cond_codec':
      return (
        <SelectField
          label="Codec"
          value={String(config.codec ?? 'hevc')}
          options={[
            { value: 'hevc', label: 'HEVC / H.265' },
            { value: 'h264', label: 'H.264 / AVC' },
            { value: 'av1', label: 'AV1' },
            { value: 'vp9', label: 'VP9' },
            { value: 'mpeg2', label: 'MPEG-2' },
          ]}
          onChange={v => set('codec', v)}
        />
      )

    case 'cond_hdr_type':
      return (
        <SelectField
          label="HDR Type"
          value={String(config.hdr_type ?? 'hdr10')}
          options={[
            { value: 'hdr10', label: 'HDR10' },
            { value: 'dv', label: 'Dolby Vision' },
            { value: 'hlg', label: 'HLG' },
            { value: 'sdr', label: 'SDR' },
          ]}
          onChange={v => set('hdr_type', v)}
        />
      )

    case 'cond_file_ext':
      return (
        <TextField
          label="Extensions (comma-separated)"
          value={
            Array.isArray(config.extensions)
              ? (config.extensions as string[]).join(', ')
              : String(config.extensions ?? '.mkv, .mp4')
          }
          onChange={v => set('extensions', v.split(',').map(s => s.trim().toLowerCase()))}
          placeholder=".mkv, .mp4, .mov"
        />
      )

    case 'audio_opus':
      return (
        <TextField
          label="Bitrate"
          value={String(config.bitrate ?? '192k')}
          onChange={v => set('bitrate', v)}
          placeholder="192k"
        />
      )

    case 'audio_aac':
      return (
        <TextField
          label="Bitrate"
          value={String(config.bitrate ?? '256k')}
          onChange={v => set('bitrate', v)}
          placeholder="256k"
        />
      )

    case 'audio_extract':
      return (
        <NumberField
          label="Track index (0-based)"
          value={Number(config.track_index ?? 0)}
          onChange={v => set('track_index', v)}
          min={0}
        />
      )

    case 'subtitle_extract':
      return (
        <>
          <NumberField
            label="Subtitle track index (0-based)"
            value={Number(config.track_index ?? 0)}
            onChange={v => set('track_index', v)}
            min={0}
          />
          <SelectField
            label="Output format"
            value={String(config.format ?? 'srt')}
            options={[
              { value: 'srt', label: 'SRT (.srt)' },
              { value: 'ass', label: 'ASS/SSA (.ass)' },
              { value: 'vtt', label: 'WebVTT (.vtt)' },
            ]}
            onChange={v => set('format', v)}
          />
        </>
      )

    case 'subtitle_convert':
      return (
        <>
          <SelectField
            label="Input format"
            value={String(config.input_format ?? 'srt')}
            options={[
              { value: 'srt', label: 'SRT' },
              { value: 'ass', label: 'ASS/SSA' },
              { value: 'vtt', label: 'WebVTT' },
            ]}
            onChange={v => set('input_format', v)}
          />
          <SelectField
            label="Output format"
            value={String(config.output_format ?? 'ass')}
            options={[
              { value: 'srt', label: 'SRT' },
              { value: 'ass', label: 'ASS/SSA' },
              { value: 'vtt', label: 'WebVTT' },
            ]}
            onChange={v => set('output_format', v)}
          />
        </>
      )

    case 'subtitle_embed':
      return (
        <>
          <TextField
            label="Subtitle file path"
            value={String(config.subtitle_path ?? '')}
            onChange={v => set('subtitle_path', v)}
            placeholder="\\\\NAS\\subs\\movie.srt"
          />
          <TextField
            label="Language code (ISO 639-2)"
            value={String(config.language ?? 'eng')}
            onChange={v => set('language', v)}
            placeholder="eng"
          />
          <div>
            <label className="flex items-center gap-2 text-xs text-th-text-muted cursor-pointer">
              <input
                type="checkbox"
                checked={Boolean(config.default_track)}
                onChange={e => set('default_track', e.target.checked)}
                className="accent-blue-600"
              />
              Set as default subtitle track
            </label>
          </div>
        </>
      )

    case 'subtitle_burn':
      return (
        <>
          <TextField
            label="Subtitle file path"
            value={String(config.subtitle_path ?? '')}
            onChange={v => set('subtitle_path', v)}
            placeholder="\\\\NAS\\subs\\movie.srt"
          />
          <NumberField
            label="Font size (px)"
            value={Number(config.font_size ?? 24)}
            onChange={v => set('font_size', v)}
            min={8}
            max={96}
          />
          <SelectField
            label="Position"
            value={String(config.position ?? 'bottom')}
            options={[
              { value: 'bottom', label: 'Bottom center' },
              { value: 'top', label: 'Top center' },
            ]}
            onChange={v => set('position', v)}
          />
        </>
      )

    case 'output_move':
    case 'output_copy':
      return (
        <TextField
          label="Destination path"
          value={String(config.destination_path ?? '')}
          onChange={v => set('destination_path', v)}
          placeholder="\\\\NAS\\encoded\\"
        />
      )

    case 'output_rename':
      return (
        <TextField
          label="Filename pattern"
          value={String(config.pattern ?? '{{.SOURCE_NAME}}_encoded')}
          onChange={v => set('pattern', v)}
          placeholder="{{.SOURCE_NAME}}_encoded"
        />
      )

    case 'output_set_path':
      return (
        <TextField
          label="Output path"
          value={String(config.path ?? '')}
          onChange={v => set('path', v)}
          placeholder="\\\\NAS\\output\\"
        />
      )

    case 'notify_webhook': {
      const webhookOptions = [
        { value: '', label: 'Select webhook…' },
        ...webhooks.map(w => ({ value: w.id, label: `${w.name} (${w.provider})` })),
      ]
      return (
        <>
          <SelectField
            label="Webhook"
            value={String(config.webhook_id ?? '')}
            options={webhookOptions}
            onChange={v => set('webhook_id', v)}
          />
          <TextField
            label="Message template"
            value={String(config.message_template ?? 'Job completed: {{.SOURCE_NAME}}')}
            onChange={v => set('message_template', v)}
          />
        </>
      )
    }

    case 'notify_discord':
    case 'notify_teams':
    case 'notify_slack': {
      const hasChannel = nodeType === 'notify_slack'
      return (
        <>
          <TextField
            label="Webhook URL"
            value={String(config.webhook_url ?? '')}
            onChange={v => set('webhook_url', v)}
            placeholder="https://…"
          />
          {nodeType === 'notify_discord' && (
            <TextField
              label="Message"
              value={String(config.message ?? 'Encoding complete: {{.SOURCE_NAME}}')}
              onChange={v => set('message', v)}
            />
          )}
          {hasChannel && (
            <TextField
              label="Channel"
              value={String(config.channel ?? '#encoding')}
              onChange={v => set('channel', v)}
            />
          )}
        </>
      )
    }

    case 'template_run':
    case 'template_frameserver': {
      const isFrameserver = nodeType === 'template_frameserver'
      const filtered = isFrameserver
        ? templates.filter(t => t.type === 'avs' || t.type === 'vpy')
        : templates.filter(t => t.type === 'bat')
      const tplOptions = [
        { value: '', label: 'Select template…' },
        ...filtered.map(t => ({ value: t.id, label: t.name })),
      ]
      return (
        <SelectField
          label="Template"
          value={String(config.template_id ?? '')}
          options={tplOptions}
          onChange={v => set('template_id', v)}
        />
      )
    }

    case 'flow_chunk_split':
      return (
        <>
          <NumberField
            label="Chunk size (frames)"
            value={Number(config.chunk_size_frames ?? 1000)}
            onChange={v => set('chunk_size_frames', v)}
            min={1}
          />
          <NumberField
            label="Overlap (frames)"
            value={Number(config.overlap_frames ?? 0)}
            onChange={v => set('overlap_frames', v)}
            min={0}
          />
        </>
      )

    case 'flow_fail':
      return (
        <TextField
          label="Error message"
          value={String(config.error_message ?? 'Flow failed')}
          onChange={v => set('error_message', v)}
        />
      )

    case 'flow_delay':
      return (
        <NumberField
          label="Delay (seconds)"
          value={Number(config.seconds ?? 5)}
          onChange={v => set('seconds', v)}
          min={0}
        />
      )

    case 'encode_twopass': {
      const codec = String(config.codec ?? 'x265')
      const presetOptions = codec === 'x264'
        ? X264_PRESETS.map(p => ({ value: p, label: p }))
        : X265_PRESETS.map(p => ({ value: p, label: p }))
      const profileOptions = codec === 'x264'
        ? X264_PROFILES.map(p => ({ value: p, label: p }))
        : X265_PROFILES.map(p => ({ value: p, label: p }))
      return (
        <>
          <SelectField
            label="Codec"
            value={codec}
            options={TWOPASS_CODEC_OPTIONS}
            onChange={v => set('codec', v)}
          />
          <NumberField
            label="Target bitrate (kbps)"
            value={Number(config.bitrate ?? 4000)}
            onChange={v => set('bitrate', v)}
            min={100}
            step={100}
          />
          <SelectField
            label="Preset"
            value={String(config.preset ?? 'slow')}
            options={presetOptions}
            onChange={v => set('preset', v)}
          />
          <SelectField
            label="Profile"
            value={String(config.profile ?? 'main10')}
            options={profileOptions}
            onChange={v => set('profile', v)}
          />
        </>
      )
    }

    case 'encode_twopass_x264':
      return (
        <>
          <NumberField
            label="Target bitrate (kbps)"
            value={Number(config.bitrate ?? 6000)}
            onChange={v => set('bitrate', v)}
            min={100}
            step={100}
          />
          <SelectField
            label="Preset"
            value={String(config.preset ?? 'medium')}
            options={X264_PRESETS.map(p => ({ value: p, label: p }))}
            onChange={v => set('preset', v)}
          />
          <SelectField
            label="Profile"
            value={String(config.profile ?? 'high')}
            options={X264_PROFILES.map(p => ({ value: p, label: p }))}
            onChange={v => set('profile', v)}
          />
        </>
      )

    case 'encode_vmaf_target': {
      const targetVmaf = Number(config.target_vmaf ?? 95)
      return (
        <>
          <div>
            <label className="block text-xs text-th-text-muted mb-1">
              Target VMAF — <span className="font-semibold text-th-text">{targetVmaf}</span>{' '}
              <span className="opacity-60">(higher = better quality)</span>
            </label>
            <input
              type="range"
              min={80}
              max={100}
              step={1}
              value={targetVmaf}
              onChange={e => set('target_vmaf', Number(e.target.value))}
              className="w-full accent-teal-500"
            />
            <div className="flex justify-between text-xs text-th-text-subtle mt-0.5">
              <span>80 minimum</span>
              <span>100 lossless</span>
            </div>
          </div>
          <SelectField
            label="Codec"
            value={String(config.codec ?? 'x265')}
            options={TWOPASS_CODEC_OPTIONS}
            onChange={v => set('codec', v)}
          />
          <SelectField
            label="Preset"
            value={String(config.preset ?? 'slow')}
            options={X265_PRESETS.map(p => ({ value: p, label: p }))}
            onChange={v => set('preset', v)}
          />
          <NumberField
            label="CRF minimum (best quality)"
            value={Number(config.crf_min ?? 15)}
            onChange={v => set('crf_min', v)}
            min={0}
            max={51}
          />
          <NumberField
            label="CRF maximum (smallest file)"
            value={Number(config.crf_max ?? 28)}
            onChange={v => set('crf_max', v)}
            min={0}
            max={51}
          />
          <NumberField
            label="Max iterations"
            value={Number(config.max_iterations ?? 5)}
            onChange={v => set('max_iterations', v)}
            min={1}
            max={20}
          />
        </>
      )
    }

    case 'analyze_vmaf':
      return (
        <>
          <SelectField
            label="VMAF model"
            value={String(config.model ?? 'vmaf_v0.6.1')}
            options={[
              { value: 'vmaf_v0.6.1', label: 'vmaf_v0.6.1 (default)' },
              { value: 'vmaf_4k_v0.6.1', label: 'vmaf_4k_v0.6.1 (4K)' },
              { value: 'vmaf_b_v0.6.3', label: 'vmaf_b_v0.6.3 (phone)' },
            ]}
            onChange={v => set('model', v)}
          />
          <TextField
            label="Reference path (optional — A/B compare)"
            value={String(config.reference_path ?? '')}
            onChange={v => set('reference_path', v)}
            placeholder="Leave blank to compare against source"
          />
        </>
      )

    case 'input_file':
      return (
        <TextField
          label="File path"
          value={String(config.path ?? '')}
          onChange={v => set('path', v)}
          placeholder="\\\\NAS\\source\\video.mkv"
        />
      )

    case 'input_folder':
      return (
        <>
          <TextField
            label="Folder path"
            value={String(config.folder_path ?? '')}
            onChange={v => set('folder_path', v)}
            placeholder="\\\\NAS\\source\\"
          />
          <TextField
            label="File pattern"
            value={String(config.file_pattern ?? '*.mkv')}
            onChange={v => set('file_pattern', v)}
            placeholder="*.mkv"
          />
        </>
      )

    case 'flow_concat':
      return (
        <SelectField
          label="Strategy"
          value={String(config.strategy ?? 'concat')}
          options={[
            { value: 'concat', label: 'Concat (ordered)' },
            { value: 'mux', label: 'Multiplex streams' },
          ]}
          onChange={v => set('strategy', v)}
        />
      )

    default:
      return <p className="text-xs text-th-text-subtle italic">No configuration required.</p>
  }
}

// ---------------------------------------------------------------------------
// Main panel
// ---------------------------------------------------------------------------

export default function NodeConfigPanel({ node, onUpdate, onClose }: Props) {
  const nodeData = node.data as unknown as FlowNodeData
  const tpl = NODE_REGISTRY_MAP.get(nodeData.nodeType)
  const color = CATEGORY_COLORS[nodeData.category] ?? '#6b7280'

  const [localConfig, setLocalConfig] = useState<Record<string, unknown>>(
    () => ({ ...(nodeData.config ?? {}) })
  )
  const [templates, setTemplates] = useState<Template[]>([])
  const [webhooks, setWebhooks] = useState<Webhook[]>([])

  // Load templates + webhooks lazily when panel is shown
  useEffect(() => {
    const needsTemplates =
      nodeData.nodeType === 'template_run' || nodeData.nodeType === 'template_frameserver'
    const needsWebhooks = nodeData.nodeType === 'notify_webhook'
    if (needsTemplates) {
      api.listTemplates().then(setTemplates).catch(() => {})
    }
    if (needsWebhooks) {
      api.listWebhooks().then(setWebhooks).catch(() => {})
    }
  }, [nodeData.nodeType])

  const handleSave = () => {
    onUpdate(node.id, localConfig)
  }

  return (
    <div className="flex flex-col h-full" style={{ minWidth: 260, maxWidth: 300 }}>
      {/* Header */}
      <div
        style={{ background: color }}
        className="flex items-center justify-between px-3 py-2.5 flex-shrink-0"
      >
        <div className="flex items-center gap-2">
          <span style={{ fontSize: 16 }}>{nodeData.icon}</span>
          <span className="text-white font-semibold text-sm">{nodeData.label}</span>
        </div>
        <button
          onClick={onClose}
          className="text-white/70 hover:text-white text-lg leading-none"
          title="Close"
        >
          ×
        </button>
      </div>

      {/* Description */}
      {tpl?.description && (
        <div className="px-3 py-2 bg-th-surface-muted border-b border-th-border text-xs text-th-text-muted flex-shrink-0">
          {tpl.description}
        </div>
      )}

      {/* Config fields */}
      <div className="flex-1 overflow-y-auto px-3 py-3 space-y-3 bg-th-surface">
        <ConfigFields
          nodeType={nodeData.nodeType}
          config={localConfig}
          onChange={setLocalConfig}
          templates={templates}
          webhooks={webhooks}
        />
      </div>

      {/* Save button */}
      <div className="px-3 py-2.5 border-t border-th-border bg-th-surface flex-shrink-0">
        <button
          onClick={handleSave}
          style={{ background: color }}
          className="w-full text-white text-sm font-medium py-1.5 rounded hover:opacity-90 transition-opacity"
        >
          Apply Changes
        </button>
      </div>
    </div>
  )
}
