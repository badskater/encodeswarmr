import { useState, useEffect } from 'react'
import * as api from '../api/client'
import type { SceneData } from '../types'

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface Props {
  // chunksText mode — render a coloured bar from raw "start,end\n…" text.
  chunksText?: string
  totalFrames?: number
  // sourceId mode — fetch scene boundaries from the API and render a
  // timeline with timecode labels.
  sourceId?: string
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

interface ChunkRange {
  start: number
  end: number
}

function parseChunks(text: string): ChunkRange[] {
  const lines = text.trim().split('\n').filter(l => l.trim() !== '')
  const chunks: ChunkRange[] = []
  for (const line of lines) {
    const parts = line.split(',').map(s => parseInt(s.trim(), 10))
    if (parts.length === 2 && !isNaN(parts[0]) && !isNaN(parts[1])) {
      chunks.push({ start: parts[0], end: parts[1] })
    }
  }
  return chunks
}

// Format a frame number as a HH:MM:SS.ff timecode string.
function framesToTimecode(frame: number, fps: number): string {
  const totalSec = frame / fps
  const h = Math.floor(totalSec / 3600)
  const m = Math.floor((totalSec % 3600) / 60)
  const s = Math.floor(totalSec % 60)
  const f = Math.round((totalSec % 1) * fps)
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}.${String(f).padStart(2, '0')}`
}

// ---------------------------------------------------------------------------
// Scene timeline sub-component (sourceId mode)
// ---------------------------------------------------------------------------

function SceneTimeline({ data }: { data: SceneData }) {
  const { scenes, total_frames, fps } = data

  if (scenes.length === 0) {
    return <p className="text-xs text-th-text-muted">No scene boundaries detected.</p>
  }

  // How many labels we can show without crowding (heuristic).
  const maxLabels = 12
  const step = Math.max(1, Math.ceil(scenes.length / maxLabels))

  return (
    <div className="space-y-1">
      {/* Timeline bar with scene-cut markers */}
      <div className="relative w-full h-6 bg-th-surface-muted rounded overflow-hidden border border-th-border-subtle">
        {/* Coloured segments between consecutive scene cuts */}
        {scenes.map((scene, i) => {
          const nextFrame = i + 1 < scenes.length ? scenes[i + 1].frame : total_frames
          const left = (scene.frame / total_frames) * 100
          const width = ((nextFrame - scene.frame) / total_frames) * 100
          const hue = (i * 137) % 360
          return (
            <div
              key={scene.frame}
              title={`Scene ${i + 1}: frame ${scene.frame} — ${scene.timecode}`}
              className="absolute h-full"
              style={{
                left: `${left}%`,
                width: `${width}%`,
                backgroundColor: `hsl(${hue}, 55%, 45%)`,
              }}
            />
          )
        })}
        {/* Vertical cut markers */}
        {scenes.map(scene => {
          const left = (scene.frame / total_frames) * 100
          return (
            <div
              key={`marker-${scene.frame}`}
              className="absolute top-0 h-full w-px bg-black/30"
              style={{ left: `${left}%` }}
            />
          )
        })}
      </div>

      {/* Timecode labels below the bar */}
      <div className="relative w-full h-4">
        {scenes.map((scene, i) => {
          if (i % step !== 0) return null
          const left = (scene.frame / total_frames) * 100
          return (
            <span
              key={`label-${scene.frame}`}
              className="absolute text-[10px] text-th-text-muted -translate-x-1/2"
              style={{ left: `${left}%` }}
            >
              {scene.timecode}
            </span>
          )
        })}
      </div>

      <p className="text-xs text-th-text-muted">
        {scenes.length} scene cut{scenes.length !== 1 ? 's' : ''} detected
        {fps > 0 ? ` @ ${fps.toFixed(3)} fps` : ''}
        {' '}— {framesToTimecode(total_frames, fps)} total
      </p>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Chunk colour bar sub-component (chunksText mode)
// ---------------------------------------------------------------------------

function ChunkBar({ chunksText, totalFrames }: { chunksText: string; totalFrames?: number }) {
  const chunks = parseChunks(chunksText)

  if (chunks.length === 0) return null

  const totalDuration = totalFrames != null
    ? totalFrames
    : Math.max(...chunks.map(c => c.end)) + 1

  const avgFrames = Math.round(chunks.reduce((sum, c) => sum + (c.end - c.start + 1), 0) / chunks.length)

  return (
    <div className="space-y-1">
      <div className="w-full h-6 rounded overflow-hidden flex">
        {chunks.map((chunk, i) => {
          const width = ((chunk.end - chunk.start + 1) / totalDuration) * 100
          const hue = (i * 137) % 360
          return (
            <div
              key={i}
              title={`Chunk ${i}: frames ${chunk.start}–${chunk.end}`}
              style={{ width: `${width}%`, backgroundColor: `hsl(${hue}, 60%, 50%)` }}
            />
          )
        })}
      </div>
      <p className="text-xs text-th-text-muted">
        {chunks.length} chunk{chunks.length !== 1 ? 's' : ''}, ~{avgFrames.toLocaleString()} frames each (avg)
      </p>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main export
// ---------------------------------------------------------------------------

export default function ChunkBoundaryPreview({ chunksText, totalFrames, sourceId }: Props) {
  const [sceneData, setSceneData] = useState<SceneData | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!sourceId) return
    setLoading(true)
    setError('')
    setSceneData(null)
    api.getSourceScenes(sourceId)
      .then(data => setSceneData(data))
      .catch(e => setError(e instanceof Error ? e.message : 'Failed to load scene data'))
      .finally(() => setLoading(false))
  }, [sourceId])

  // sourceId mode — show the scene timeline fetched from the API.
  if (sourceId) {
    if (loading) {
      return <p className="text-xs text-th-text-muted">Loading scene data…</p>
    }
    if (error) {
      return <p className="text-xs text-red-500">{error}</p>
    }
    if (!sceneData) return null
    return <SceneTimeline data={sceneData} />
  }

  // chunksText mode — render the colour bar from parsed text.
  if (chunksText) {
    return <ChunkBar chunksText={chunksText} totalFrames={totalFrames} />
  }

  return null
}
