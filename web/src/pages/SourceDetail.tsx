import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import * as api from '../api/client'
import type { Source, AnalysisResult } from '../types'
import StatusBadge from '../components/StatusBadge'

function fmtBytes(n: number) {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + ' GB'
  if (n >= 1e6) return (n / 1e6).toFixed(1) + ' MB'
  if (n >= 1e3) return (n / 1e3).toFixed(1) + ' KB'
  return n + ' B'
}

function fmtDuration(sec: number | null) {
  if (sec == null) return '—'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  return h > 0 ? `${h}h ${m}m ${s}s` : `${m}m ${s}s`
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex py-2 border-b border-gray-100 last:border-0">
      <span className="w-40 text-sm text-gray-500 shrink-0">{label}</span>
      <span className="text-sm text-gray-900">{value}</span>
    </div>
  )
}

export default function SourceDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [source, setSource] = useState<Source | null>(null)
  const [analysis, setAnalysis] = useState<AnalysisResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!id) return
    Promise.all([api.getSource(id), api.getAnalysis(id).catch(() => null)])
      .then(([s, a]) => { setSource(s); setAnalysis(a) })
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false))
  }, [id])

  if (loading) return <p className="text-gray-500">Loading…</p>
  if (error) return <p className="text-red-600">{error}</p>
  if (!source) return <p className="text-gray-500">Source not found</p>

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <button onClick={() => navigate('/sources')} className="text-blue-600 hover:underline text-sm">← Sources</button>
        <h1 className="text-2xl font-bold text-gray-900">{source.filename}</h1>
      </div>

      <div className="bg-white rounded-lg shadow p-4 space-y-1">
        <h2 className="text-sm font-semibold text-gray-700 mb-2">Source Info</h2>
        <Row label="Filename" value={source.filename} />
        <Row label="Path" value={<span className="font-mono text-xs break-all">{source.path}</span>} />
        <Row label="Size" value={fmtBytes(source.size_bytes)} />
        <Row label="Duration" value={fmtDuration(source.duration_sec)} />
        <Row label="State" value={<StatusBadge status={source.state} />} />
        <Row label="VMAF Score" value={source.vmaf_score != null ? source.vmaf_score.toFixed(2) : '—'} />
      </div>

      {analysis && (
        <div className="bg-white rounded-lg shadow p-4 space-y-1">
          <h2 className="text-sm font-semibold text-gray-700 mb-2">Analysis Results</h2>
          <Row label="VMAF Score" value={analysis.vmaf_score != null ? analysis.vmaf_score.toFixed(2) : '—'} />
          <Row label="PSNR" value={analysis.psnr != null ? analysis.psnr.toFixed(2) + ' dB' : '—'} />
          <Row label="SSIM" value={analysis.ssim != null ? analysis.ssim.toFixed(4) : '—'} />
          <Row label="Resolution" value={analysis.width && analysis.height ? `${analysis.width}×${analysis.height}` : '—'} />
          <Row label="Duration" value={fmtDuration(analysis.duration_sec)} />
          <Row label="Frame Count" value={analysis.frame_count != null ? analysis.frame_count.toLocaleString() : '—'} />
        </div>
      )}
    </div>
  )
}
