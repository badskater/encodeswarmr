import { useEffect, useRef, useState, useCallback } from 'react'

interface LogEntry {
  id: number
  task_id: string
  job_id: string
  stream: string
  level: string
  message: string
  metadata?: Record<string, unknown>
  timestamp: string
}

interface ProgressInfo {
  fps?: number
  frame?: number
  time?: string
  speed?: string
  percent?: number
}

interface Props {
  taskId: string
  /** If true, the component creates a WebSocket connection for live streaming */
  live?: boolean
  /** If provided, fetch historical logs after this log ID */
  afterId?: number
}

// ANSI escape code sequence regex
const ANSI_RE = /\x1b\[[0-9;]*m/g

// Strip ANSI colour codes for plain display (no colour rendering needed for
// server logs; ffmpeg colours are informational noise in a small viewer).
function stripAnsi(s: string): string {
  return s.replace(ANSI_RE, '')
}

// Extract ffmpeg-style progress fields from a log line.
// Example: "frame=  120 fps= 24 q=-1.0 size=  512kB time=00:00:05.00 speed=1.5x"
function extractProgress(msg: string): ProgressInfo | null {
  const fpsM = msg.match(/fps=\s*([\d.]+)/)
  const frameM = msg.match(/frame=\s*(\d+)/)
  const timeM = msg.match(/time=(\d{2}:\d{2}:\d{2}\.\d{2})/)
  const speedM = msg.match(/speed=\s*([\d.]+x)/)

  if (!fpsM && !frameM && !timeM) return null

  return {
    fps: fpsM ? parseFloat(fpsM[1]) : undefined,
    frame: frameM ? parseInt(frameM[1]) : undefined,
    time: timeM ? timeM[1] : undefined,
    speed: speedM ? speedM[1] : undefined,
  }
}

export default function TaskLogViewer({ taskId, live = false, afterId }: Props) {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [search, setSearch] = useState('')
  const [stickToBottom, setStickToBottom] = useState(true)
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState('')
  const [progress, setProgress] = useState<ProgressInfo | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)
  const logContainerRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WebSocket | null>(null)

  // Scroll to bottom when new logs arrive if stickToBottom is active.
  useEffect(() => {
    if (stickToBottom && bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' })
    }
  }, [logs, stickToBottom])

  // Detect when user manually scrolls up — disable stick-to-bottom.
  const handleScroll = useCallback(() => {
    const el = logContainerRef.current
    if (!el) return
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40
    setStickToBottom(atBottom)
  }, [])

  useEffect(() => {
    if (!live) return

    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const url = `${protocol}://${window.location.host}/api/v1/tasks/${taskId}/logs/stream${afterId != null ? `?after_id=${afterId}` : ''}`
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => setConnected(true)
    ws.onclose = () => {
      setConnected(false)
      wsRef.current = null
    }
    ws.onerror = () => {
      setError('WebSocket connection failed')
      setConnected(false)
    }
    ws.onmessage = (evt) => {
      try {
        const entry: LogEntry = JSON.parse(evt.data)
        setLogs(prev => [...prev, entry])
        // Update progress bar from ffmpeg output.
        const prog = extractProgress(entry.message)
        if (prog) setProgress(prog)
      } catch {
        // Ignore malformed frames.
      }
    }

    return () => {
      ws.close()
      wsRef.current = null
    }
  }, [taskId, live, afterId])

  const filteredLogs = search
    ? logs.filter(l => l.message.toLowerCase().includes(search.toLowerCase()))
    : logs

  const levelColor = (level: string) => {
    switch (level.toLowerCase()) {
      case 'error': return 'text-red-500'
      case 'warn': return 'text-yellow-500'
      case 'debug': return 'text-th-text-muted'
      default: return 'text-th-text'
    }
  }

  return (
    <div className="flex flex-col h-full min-h-0">
      {/* Toolbar */}
      <div className="flex items-center gap-2 px-2 py-1.5 border-b border-th-border bg-th-surface-muted">
        <input
          type="text"
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder="Filter logs…"
          className="flex-1 bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-xs text-th-text"
        />
        {live && (
          <span className={`text-xs font-medium px-2 py-0.5 rounded ${connected ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
            {connected ? 'Live' : 'Disconnected'}
          </span>
        )}
        <button
          onClick={() => setStickToBottom(v => !v)}
          className={`text-xs px-2 py-0.5 rounded border ${stickToBottom ? 'border-blue-400 text-blue-600 bg-blue-50' : 'border-th-border text-th-text-muted'}`}
          title="Stick to bottom"
        >
          ↓ Follow
        </button>
      </div>

      {/* Progress bar (ffmpeg) */}
      {progress && (
        <div className="flex items-center gap-3 px-2 py-1 bg-th-surface border-b border-th-border text-xs text-th-text-muted">
          {progress.fps != null && <span>FPS: <span className="text-th-text font-mono">{progress.fps.toFixed(1)}</span></span>}
          {progress.frame != null && <span>Frame: <span className="text-th-text font-mono">{progress.frame}</span></span>}
          {progress.time && <span>Time: <span className="text-th-text font-mono">{progress.time}</span></span>}
          {progress.speed && <span>Speed: <span className="text-th-text font-mono">{progress.speed}</span></span>}
        </div>
      )}

      {error && <p className="text-red-600 text-xs px-2 py-1">{error}</p>}

      {/* Log pane */}
      <div
        ref={logContainerRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto font-mono text-xs bg-gray-950 text-gray-200 p-2 space-y-0.5"
        style={{ minHeight: '200px' }}
      >
        {filteredLogs.length === 0 && (
          <p className="text-gray-500 italic">{search ? 'No matching log lines.' : 'No logs yet.'}</p>
        )}
        {filteredLogs.map(l => (
          <div key={l.id} className="flex gap-2 leading-5">
            <span className="text-gray-500 flex-shrink-0 select-none">
              {new Date(l.timestamp).toLocaleTimeString()}
            </span>
            <span className={`flex-shrink-0 w-5 text-center select-none ${levelColor(l.level)}`}>
              {l.level.charAt(0).toUpperCase()}
            </span>
            <span className="text-gray-400 flex-shrink-0 select-none">[{l.stream}]</span>
            <span className={levelColor(l.level)}>{stripAnsi(l.message)}</span>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  )
}
