import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import type { Source, AnalysisResult, AnalysisFramePoint } from '../types'
import StatusBadge from '../components/StatusBadge'

function fmtBytes(n: number) {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + ' GB'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + ' MB'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + ' KB'
  return n + ' B'
}

function fmtDuration(sec: number | null | undefined) {
  if (sec == null) return '—'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  return h > 0 ? `${h}h ${m}m ${s}s` : `${m}m ${s}s`
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex py-2 border-b border-th-border-subtle last:border-0">
      <span className="w-40 text-sm text-th-text-muted shrink-0">{label}</span>
      <span className="text-sm text-th-text">{value}</span>
    </div>
  )
}

function VmafSparkline({ data }: { data: AnalysisFramePoint[] }) {
  if (!data.length) return null

  const W = 600
  const H = 60
  const PAD = 2

  // Downsample to at most 300 points for rendering performance.
  const step = Math.ceil(data.length / 300)
  const sampled = data.filter((_, i) => i % step === 0)

  const scores = sampled.map(p => p.score ?? 0)
  const minScore = Math.min(...scores)
  const maxScore = Math.max(...scores)
  const range = maxScore - minScore || 1

  const xScale = (W - PAD * 2) / (sampled.length - 1 || 1)
  const yScale = (H - PAD * 2) / range

  const points = sampled
    .map((p, i) => `${PAD + i * xScale},${H - PAD - ((p.score ?? 0) - minScore) * yScale}`)
    .join(' ')

  // Draw a reference line at score=90 if it falls within the visible range.
  const refY = H - PAD - (90 - minScore) * yScale
  const showRef = refY > PAD && refY < H - PAD

  return (
    <div className="mt-3">
      <p className="text-xs text-th-text-muted mb-1">
        Per-frame VMAF ({sampled.length.toLocaleString()} samples
        {data.length > sampled.length ? `, 1 per ${step} frames` : ''})
      </p>
      <svg viewBox={`0 0 ${W} ${H}`} className="w-full h-16 bg-th-surface-muted rounded">
        {showRef && (
          <line
            x1={PAD} y1={refY} x2={W - PAD} y2={refY}
            stroke="currentColor" strokeOpacity="0.25" strokeDasharray="4 3"
            className="text-th-border"
          />
        )}
        <polyline
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          className="text-blue-500"
          points={points}
        />
      </svg>
      {showRef && (
        <p className="text-xs text-th-text-subtle mt-0.5">Dashed line = score 90 (reference quality)</p>
      )}
    </div>
  )
}

const HDR_TYPES = ['', 'hdr10', 'hdr10plus', 'dolby_vision', 'hlg']

function hdrLabel(hdrType: string, dvProfile: number): string {
  if (hdrType === 'dolby_vision') return dvProfile > 0 ? `Dolby Vision Profile ${dvProfile}` : 'Dolby Vision'
  if (hdrType === 'hdr10plus') return 'HDR10+'
  if (hdrType === 'hdr10') return 'HDR10'
  if (hdrType === 'hlg') return 'HLG'
  if (hdrType === '') return 'Unknown / SDR'
  return hdrType
}

export default function SourceDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [source, setSource] = useState<Source | null>(null)
  const [analysis, setAnalysis] = useState<AnalysisResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [detectingHDR, setDetectingHDR] = useState(false)
  const [showHDROverride, setShowHDROverride] = useState(false)
  const [overrideHDRType, setOverrideHDRType] = useState('')
  const [overrideDVProfile, setOverrideDVProfile] = useState(0)
  const [savingHDR, setSavingHDR] = useState(false)

  useEffect(() => {
    if (!id) return
    Promise.all([api.getSource(id), api.getAnalysis(id).catch(() => null)])
      .then(([s, a]) => { setSource(s); setAnalysis(a) })
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false))
  }, [id])

  const handleHDRDetect = async () => {
    if (!id) return
    setDetectingHDR(true)
    setError('')
    try {
      await api.hdrDetectSource(id)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to trigger HDR detection')
    } finally {
      setDetectingHDR(false)
    }
  }

  const handleHDROverrideSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!id) return
    setSavingHDR(true)
    setError('')
    try {
      const updated = await api.updateSourceHDR(id, overrideHDRType, overrideDVProfile)
      setSource(updated)
      setShowHDROverride(false)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to update HDR metadata')
    } finally {
      setSavingHDR(false)
    }
  }

  if (loading) return <p className="text-th-text-muted">Loading…</p>
  if (error) return <p className="text-red-600">{error}</p>
  if (!source) return <p className="text-th-text-muted">Source not found</p>

  const sum = analysis?.summary

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <button onClick={() => navigate('/sources')} className="text-blue-600 hover:underline text-sm">← Sources</button>
        <h1 className="text-2xl font-bold text-th-text">{source.filename}</h1>
      </div>

      <div className="bg-th-surface rounded-lg shadow p-4 space-y-1">
        <h2 className="text-sm font-semibold text-th-text-secondary mb-2">Source Info</h2>
        <Row label="Filename" value={source.filename} />
        <Row label="Path" value={<span className="font-mono text-xs break-all">{source.path}</span>} />
        <Row label="Size" value={fmtBytes(source.size_bytes)} />
        <Row label="Duration" value={fmtDuration(source.duration_sec)} />
        <Row label="State" value={<StatusBadge status={source.state} />} />
        <Row label="VMAF Score" value={source.vmaf_score != null ? source.vmaf_score.toFixed(2) : '—'} />
        <Row label="HDR Type" value={
          <span className="flex items-center gap-2">
            {hdrLabel(source.hdr_type, source.dv_profile)}
            <button
              onClick={handleHDRDetect}
              disabled={detectingHDR}
              className="text-xs px-2 py-0.5 rounded disabled:opacity-50"
              style={{ backgroundColor: 'var(--th-badge-assigned-bg)', color: 'var(--th-badge-assigned-text)' }}
            >
              {detectingHDR ? 'Queuing…' : 'Detect'}
            </button>
            <button
              onClick={() => {
                setOverrideHDRType(source.hdr_type)
                setOverrideDVProfile(source.dv_profile)
                setShowHDROverride(v => !v)
              }}
              className="text-xs px-2 py-0.5 rounded"
              style={{ backgroundColor: 'var(--th-badge-neutral-bg)', color: 'var(--th-badge-neutral-text)' }}
            >
              Override
            </button>
          </span>
        } />
        {showHDROverride && (
          <form onSubmit={handleHDROverrideSave} className="flex items-end gap-2 py-2 pl-40">
            <div>
              <label className="block text-xs text-th-text-muted mb-1">HDR Type</label>
              <select
                value={overrideHDRType}
                onChange={e => setOverrideHDRType(e.target.value)}
                className="bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-sm text-th-text"
              >
                {HDR_TYPES.map(t => (
                  <option key={t} value={t}>{t === '' ? 'Unknown / SDR' : t}</option>
                ))}
              </select>
            </div>
            {overrideHDRType === 'dolby_vision' && (
              <div>
                <label className="block text-xs text-th-text-muted mb-1">DV Profile</label>
                <input
                  type="number"
                  min={0}
                  max={9}
                  value={overrideDVProfile}
                  onChange={e => setOverrideDVProfile(parseInt(e.target.value) || 0)}
                  className="w-20 bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-sm text-th-text"
                />
              </div>
            )}
            <button type="submit" disabled={savingHDR}
              className="bg-blue-600 text-white px-3 py-1 rounded text-sm hover:bg-blue-700 disabled:opacity-50">
              {savingHDR ? 'Saving…' : 'Save'}
            </button>
            <button type="button" onClick={() => setShowHDROverride(false)}
              className="text-sm text-th-text-muted hover:text-th-text">
              Cancel
            </button>
          </form>
        )}
      </div>

      {analysis ? (
        <div className="bg-th-surface rounded-lg shadow p-4">
          <h2 className="text-sm font-semibold text-th-text-secondary mb-2">
            Analysis Results
            <span className="ml-2 font-normal text-th-text-subtle">({analysis.type})</span>
          </h2>

          {sum && (
            <div className="space-y-0">
              {sum.mean != null && (
                <Row label="VMAF Mean" value={sum.mean.toFixed(2)} />
              )}
              {sum.min != null && (
                <Row label="VMAF Min" value={sum.min.toFixed(2)} />
              )}
              {sum.max != null && (
                <Row label="VMAF Max" value={sum.max.toFixed(2)} />
              )}
              {sum.psnr != null && (
                <Row label="PSNR" value={sum.psnr.toFixed(2) + ' dB'} />
              )}
              {sum.ssim != null && (
                <Row label="SSIM" value={sum.ssim.toFixed(4)} />
              )}
              {sum.width != null && sum.height != null && (
                <Row label="Resolution" value={`${sum.width}×${sum.height}`} />
              )}
              {sum.duration_sec != null && (
                <Row label="Duration" value={fmtDuration(sum.duration_sec)} />
              )}
              {sum.frame_count != null && (
                <Row label="Frame Count" value={sum.frame_count.toLocaleString()} />
              )}
              {sum.codec && (
                <Row label="Codec" value={sum.codec} />
              )}
              {sum.bit_rate != null && (
                <Row label="Bit Rate" value={fmtBytes(sum.bit_rate) + '/s'} />
              )}
              {sum.scene_count != null && (
                <Row label="Scene Count" value={sum.scene_count.toLocaleString()} />
              )}
            </div>
          )}

          {analysis.frame_data && analysis.frame_data.length > 0 && (
            <VmafSparkline data={analysis.frame_data} />
          )}

          {!sum && (!analysis.frame_data || analysis.frame_data.length === 0) && (
            <p className="text-sm text-th-text-muted">No summary data available for this analysis.</p>
          )}

          <p className="text-xs text-th-text-subtle mt-3">
            Recorded {new Date(analysis.created_at).toLocaleString()}
          </p>
        </div>
      ) : (
        <div className="bg-th-surface rounded-lg shadow p-4">
          <h2 className="text-sm font-semibold text-th-text-secondary mb-1">Analysis Results</h2>
          <p className="text-sm text-th-text-muted">No analysis results yet. Run an analysis job to populate this section.</p>
        </div>
      )}
    </div>
  )
}
