import type { ComparisonResponse } from '../types'

interface Props {
  data: ComparisonResponse
}

function Gauge({ label, value, max, unit = '' }: { label: string; value: number; max: number; unit?: string }) {
  const pct = Math.min(100, Math.max(0, max > 0 ? (value / max) * 100 : 0))
  const color = pct >= 90 ? 'bg-green-500' : pct >= 70 ? 'bg-yellow-500' : 'bg-red-500'
  return (
    <div className="flex flex-col gap-1">
      <div className="flex justify-between text-xs text-th-text-muted">
        <span>{label}</span>
        <span className="font-mono font-medium text-th-text">{value.toFixed(1)}{unit}</span>
      </div>
      <div className="h-2 rounded-full bg-th-surface-muted overflow-hidden">
        <div className={`h-full rounded-full transition-all ${color}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}

function SizeBar({ label, mb, maxMb }: { label: string; mb: number; maxMb: number }) {
  const pct = maxMb > 0 ? (mb / maxMb) * 100 : 0
  return (
    <div className="flex flex-col gap-1">
      <div className="flex justify-between text-xs text-th-text-muted">
        <span>{label}</span>
        <span className="font-mono font-medium text-th-text">{mb.toFixed(0)} MB</span>
      </div>
      <div className="h-3 rounded-full bg-th-surface-muted overflow-hidden">
        <div
          className="h-full rounded-full bg-blue-500 transition-all"
          style={{ width: `${Math.min(100, pct)}%` }}
        />
      </div>
    </div>
  )
}

export default function ComparisonCard({ data }: Props) {
  const maxMb = Math.max(data.source.file_size_mb, data.output.file_size_mb, 1)

  return (
    <div className="bg-th-surface rounded-lg shadow p-4 space-y-5">
      <h2 className="text-sm font-semibold text-th-text-secondary">Output Comparison</h2>

      {/* Side-by-side metadata */}
      <div className="grid grid-cols-2 gap-4 text-sm">
        <div className="space-y-1">
          <p className="text-xs text-th-text-muted uppercase font-medium">Source</p>
          <p className="text-th-text">{data.source.file_size_mb.toFixed(0)} MB</p>
          {data.source.codec && <p className="text-xs text-th-text-muted">{data.source.codec}</p>}
          {data.source.resolution && <p className="text-xs text-th-text-muted">{data.source.resolution}</p>}
          {data.source.duration_sec > 0 && (
            <p className="text-xs text-th-text-muted">{Math.round(data.source.duration_sec)}s</p>
          )}
        </div>
        <div className="space-y-1">
          <p className="text-xs text-th-text-muted uppercase font-medium">Output</p>
          <p className="text-th-text">{data.output.file_size_mb.toFixed(0)} MB</p>
          {data.output.codec && <p className="text-xs text-th-text-muted">{data.output.codec}</p>}
          {data.output.resolution && <p className="text-xs text-th-text-muted">{data.output.resolution}</p>}
          {data.output.duration_sec > 0 && (
            <p className="text-xs text-th-text-muted">{Math.round(data.output.duration_sec)}s</p>
          )}
        </div>
      </div>

      {/* File size bar chart */}
      <div className="space-y-2">
        <SizeBar label="Source size" mb={data.source.file_size_mb} maxMb={maxMb} />
        <SizeBar label="Output size" mb={data.output.file_size_mb} maxMb={maxMb} />
      </div>

      {/* Summary stats */}
      <div className="grid grid-cols-2 gap-3 text-sm">
        <div className="bg-th-surface-muted rounded p-2 text-center">
          <p className="text-xs text-th-text-muted">Compression ratio</p>
          <p className="font-bold text-th-text">{data.compression_ratio.toFixed(2)}x</p>
        </div>
        <div className="bg-th-surface-muted rounded p-2 text-center">
          <p className="text-xs text-th-text-muted">Size reduction</p>
          <p className="font-bold text-green-500">{data.size_reduction_pct.toFixed(1)}%</p>
        </div>
      </div>

      {/* Quality gauges */}
      {(data.vmaf_score != null || data.psnr != null || data.ssim != null) && (
        <div className="space-y-3 border-t border-th-border pt-4">
          <p className="text-xs font-medium text-th-text-secondary">Quality Metrics</p>
          {data.vmaf_score != null && (
            <Gauge label="VMAF" value={data.vmaf_score} max={100} />
          )}
          {data.psnr != null && (
            <Gauge label="PSNR" value={data.psnr} max={60} unit=" dB" />
          )}
          {data.ssim != null && (
            <Gauge label="SSIM" value={data.ssim * 100} max={100} unit="%" />
          )}
        </div>
      )}
    </div>
  )
}
