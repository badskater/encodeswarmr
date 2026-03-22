import { useState, useEffect, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  ReactFlow,
  Background,
  BackgroundVariant,
  MiniMap,
  Controls,
  addEdge,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  type Connection,
  type NodeTypes,
  type OnConnectStartParams,
  Panel,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'

import * as api from '../api/client'
import type { Flow, FlowNodeData } from '../types/flow'
import { NODE_REGISTRY_MAP, CATEGORY_COLORS } from '../flow/nodeRegistry'
import FlowNode from '../flow/nodes/FlowNode'
import NodeSidebar from '../flow/NodeSidebar'
import NodeConfigPanel from '../flow/NodeConfigPanel'

// ---------------------------------------------------------------------------
// Custom node types registration
// ---------------------------------------------------------------------------

const nodeTypes: NodeTypes = {
  flowNode: FlowNode,
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeNodeId() {
  return `node_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`
}

function buildReactFlowNode(
  nodeType: string,
  position: { x: number; y: number }
): Node | null {
  const tpl = NODE_REGISTRY_MAP.get(nodeType)
  if (!tpl) return null

  const data: FlowNodeData & { inputs: number; outputs: number } = {
    label: tpl.label,
    category: tpl.category,
    description: tpl.description,
    config: { ...tpl.defaultConfig },
    nodeType: tpl.type,
    icon: tpl.icon,
    inputs: tpl.inputs,
    outputs: tpl.outputs,
  }

  return {
    id: makeNodeId(),
    type: 'flowNode',
    position,
    data: data as unknown as Record<string, unknown>,
  }
}

// ---------------------------------------------------------------------------
// Undo/Redo stack
// ---------------------------------------------------------------------------

interface Snapshot {
  nodes: Node[]
  edges: Edge[]
}

function useUndoRedo(
  nodes: Node[],
  edges: Edge[],
  setNodes: (nodes: Node[]) => void,
  setEdges: (edges: Edge[]) => void
) {
  const past = useRef<Snapshot[]>([])
  const future = useRef<Snapshot[]>([])

  const push = useCallback(() => {
    past.current.push({ nodes: [...nodes], edges: [...edges] })
    future.current = []
  }, [nodes, edges])

  const undo = useCallback(() => {
    if (past.current.length === 0) return
    future.current.push({ nodes: [...nodes], edges: [...edges] })
    const snap = past.current.pop()!
    setNodes(snap.nodes)
    setEdges(snap.edges)
  }, [nodes, edges, setNodes, setEdges])

  const redo = useCallback(() => {
    if (future.current.length === 0) return
    past.current.push({ nodes: [...nodes], edges: [...edges] })
    const snap = future.current.pop()!
    setNodes(snap.nodes)
    setEdges(snap.edges)
  }, [nodes, edges, setNodes, setEdges])

  return { push, undo, redo, canUndo: past.current.length > 0, canRedo: future.current.length > 0 }
}

// ---------------------------------------------------------------------------
// FlowEditor page
// ---------------------------------------------------------------------------

export default function FlowEditor() {
  const { id } = useParams<{ id?: string }>()
  const navigate = useNavigate()

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])

  const [flowName, setFlowName] = useState('Untitled Flow')
  const [flowDesc, setFlowDesc] = useState('')
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [loading, setLoading] = useState(!!id)
  const [selectedNode, setSelectedNode] = useState<Node | null>(null)

  const reactFlowWrapper = useRef<HTMLDivElement>(null)
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const reactFlowInstance = useRef<any>(null)

  const { push: pushSnapshot, undo, redo } = useUndoRedo(
    nodes,
    edges,
    setNodes as (n: Node[]) => void,
    setEdges as (e: Edge[]) => void
  )

  // ── Load existing flow ────────────────────────────────────────────────────
  useEffect(() => {
    if (!id) return
    setLoading(true)
    api.getFlow(id)
      .then((f: Flow) => {
        setFlowName(f.name)
        setFlowDesc(f.description ?? '')
        setNodes(f.nodes ?? [])
        setEdges(f.edges ?? [])
      })
      .catch(() => navigate('/flows'))
      .finally(() => setLoading(false))
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  // ── Connect ───────────────────────────────────────────────────────────────
  const onConnect = useCallback(
    (params: Connection) => {
      pushSnapshot()
      setEdges(eds =>
        addEdge(
          {
            ...params,
            animated: true,
            style: { strokeWidth: 2 },
          },
          eds
        )
      )
    },
    [pushSnapshot, setEdges]
  )

  // Prevent invalid connections (output→output / input→input)
  const connectingHandle = useRef<OnConnectStartParams | null>(null)
  const onConnectStart = useCallback((_event: MouseEvent | TouchEvent, params: OnConnectStartParams) => {
    connectingHandle.current = params
  }, [])
  const onConnectEnd = useCallback(() => {
    connectingHandle.current = null
  }, [])

  const isValidConnection = useCallback((connection: Connection | { source: string; target: string }) => {
    // Prevent self-loops
    if (connection.source === connection.target) return false
    return true
  }, [])

  // ── Drag-and-drop from sidebar ────────────────────────────────────────────
  const onDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'copy'
  }, [])

  const onDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      const nodeType = e.dataTransfer.getData('application/reactflow-nodetype')
      if (!nodeType || !reactFlowInstance.current || !reactFlowWrapper.current) return

      const bounds = reactFlowWrapper.current.getBoundingClientRect()
      const position = reactFlowInstance.current.screenToFlowPosition({
        x: e.clientX - bounds.left,
        y: e.clientY - bounds.top,
      })

      const newNode = buildReactFlowNode(nodeType, position)
      if (!newNode) return

      pushSnapshot()
      setNodes(nds => [...nds, newNode])
    },
    [pushSnapshot, setNodes]
  )

  // ── Select / deselect ─────────────────────────────────────────────────────
  const onNodeClick = useCallback((_: unknown, node: Node) => {
    setSelectedNode(node)
  }, [])

  const onPaneClick = useCallback(() => {
    setSelectedNode(null)
  }, [])

  // ── Update node config ────────────────────────────────────────────────────
  const handleConfigUpdate = useCallback(
    (nodeId: string, config: Record<string, unknown>) => {
      pushSnapshot()
      setNodes(nds =>
        nds.map(n => {
          if (n.id !== nodeId) return n
          return {
            ...n,
            data: {
              ...n.data,
              config,
            },
          }
        })
      )
    },
    [pushSnapshot, setNodes]
  )

  // ── Keyboard delete ───────────────────────────────────────────────────────
  const onKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Delete' || e.key === 'Backspace') {
        if (document.activeElement?.tagName === 'INPUT' || document.activeElement?.tagName === 'TEXTAREA') return
        pushSnapshot()
        setNodes(nds => nds.filter(n => !n.selected))
        setEdges(eds => eds.filter(e => !e.selected))
        setSelectedNode(null)
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'z') {
        e.preventDefault()
        undo()
      }
      if ((e.ctrlKey || e.metaKey) && (e.key === 'y' || (e.shiftKey && e.key === 'z'))) {
        e.preventDefault()
        redo()
      }
    },
    [pushSnapshot, setNodes, setEdges, undo, redo]
  )

  // ── Save ──────────────────────────────────────────────────────────────────
  const handleSave = async () => {
    setSaving(true)
    setSaveError('')
    try {
      const payload = {
        name: flowName.trim() || 'Untitled Flow',
        description: flowDesc,
        nodes,
        edges,
      }
      if (id) {
        await api.updateFlow(id, payload)
      } else {
        const created = await api.createFlow(payload)
        navigate(`/flows/editor/${created.id}`, { replace: true })
      }
    } catch (err: unknown) {
      setSaveError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  // ── Delete flow ───────────────────────────────────────────────────────────
  const handleDelete = async () => {
    if (!id) return
    if (!confirm('Delete this flow? This cannot be undone.')) return
    try {
      await api.deleteFlow(id)
      navigate('/flows')
    } catch {
      setSaveError('Delete failed')
    }
  }

  // ── MiniMap node color ────────────────────────────────────────────────────
  const minimapNodeColor = useCallback((node: Node) => {
    const cat = (node.data as unknown as FlowNodeData)?.category
    return cat ? CATEGORY_COLORS[cat] ?? '#6b7280' : '#6b7280'
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-th-text-muted text-sm">
        Loading flow…
      </div>
    )
  }

  return (
    <div
      className="flex flex-col"
      style={{ height: 'calc(100vh - 64px)' }}
      onKeyDown={onKeyDown}
      tabIndex={-1}
      // eslint-disable-next-line jsx-a11y/no-noninteractive-element-to-interactive-role
    >
      {/* ── Toolbar ─────────────────────────────────────────────────────── */}
      <div className="flex items-center gap-3 px-4 py-2 bg-th-surface border-b border-th-border flex-shrink-0 flex-wrap">
        <button
          onClick={() => navigate('/flows')}
          className="text-blue-500 hover:underline text-sm flex-shrink-0"
        >
          ← Flows
        </button>

        <div className="flex items-center gap-2 flex-1 min-w-0">
          <input
            type="text"
            value={flowName}
            onChange={e => setFlowName(e.target.value)}
            className="bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-sm text-th-text font-semibold focus:outline-none focus:ring-1 min-w-0 flex-1 max-w-xs"
            placeholder="Flow name"
          />
          <input
            type="text"
            value={flowDesc}
            onChange={e => setFlowDesc(e.target.value)}
            className="bg-th-input-bg border border-th-input-border rounded px-2 py-1 text-sm text-th-text-muted focus:outline-none focus:ring-1 min-w-0 flex-1 max-w-sm hidden md:block"
            placeholder="Description…"
          />
        </div>

        <div className="flex items-center gap-2 flex-shrink-0">
          {/* Undo / Redo */}
          <button
            onClick={undo}
            title="Undo (Ctrl+Z)"
            className="p-1.5 rounded text-th-text-muted hover:bg-th-surface-muted hover:text-th-text transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 14L4 9l5-5M4 9h11a6 6 0 010 12h-1" />
            </svg>
          </button>
          <button
            onClick={redo}
            title="Redo (Ctrl+Y)"
            className="p-1.5 rounded text-th-text-muted hover:bg-th-surface-muted hover:text-th-text transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M15 14l5-5-5-5M19 9H8a6 6 0 000 12h1" />
            </svg>
          </button>

          <div className="w-px h-5 bg-th-border" />

          {/* Test Run (disabled) */}
          <button
            disabled
            className="px-3 py-1.5 rounded text-sm font-medium bg-th-surface-muted text-th-text-subtle cursor-not-allowed"
            title="Test Run — coming soon"
          >
            ▶ Test Run
          </button>

          {/* Save */}
          <button
            onClick={handleSave}
            disabled={saving}
            className="px-3 py-1.5 rounded text-sm font-medium bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-60 transition-colors"
          >
            {saving ? 'Saving…' : id ? 'Save' : 'Save Flow'}
          </button>

          {/* Delete */}
          {id && (
            <button
              onClick={handleDelete}
              className="px-3 py-1.5 rounded text-sm font-medium bg-red-600 text-white hover:bg-red-700 transition-colors"
            >
              Delete
            </button>
          )}
        </div>

        {saveError && (
          <span className="text-xs text-red-500 flex-shrink-0">{saveError}</span>
        )}
      </div>

      {/* ── Main area ───────────────────────────────────────────────────── */}
      <div className="flex flex-1 overflow-hidden">
        {/* Sidebar */}
        <NodeSidebar />

        {/* Canvas */}
        <div ref={reactFlowWrapper} className="flex-1 relative" onDrop={onDrop} onDragOver={onDragOver}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onConnectStart={onConnectStart}
            onConnectEnd={onConnectEnd}
            isValidConnection={isValidConnection}
            onNodeClick={onNodeClick}
            onPaneClick={onPaneClick}
            nodeTypes={nodeTypes}
            onInit={inst => { reactFlowInstance.current = inst }}
            snapToGrid
            snapGrid={[12, 12]}
            fitView={nodes.length > 0}
            deleteKeyCode={null} // handled manually
            defaultEdgeOptions={{
              animated: true,
              style: { strokeWidth: 2, stroke: '#3b82f6' },
            }}
            style={{ background: 'var(--th-bg)' }}
          >
            <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="var(--th-border)" />
            <Controls
              style={{
                background: 'var(--th-surface)',
                border: '1px solid var(--th-border)',
                borderRadius: 6,
              }}
            />
            <MiniMap
              nodeColor={minimapNodeColor}
              style={{
                background: 'var(--th-surface)',
                border: '1px solid var(--th-border)',
                borderRadius: 6,
              }}
              maskColor="rgba(0,0,0,0.08)"
            />

            {/* Empty state */}
            {nodes.length === 0 && (
              <Panel position="top-center">
                <div className="mt-16 text-center pointer-events-none select-none">
                  <p className="text-th-text-muted text-sm">Drag nodes from the left panel to get started</p>
                  <p className="text-th-text-subtle text-xs mt-1">Connect nodes to build your encoding pipeline</p>
                </div>
              </Panel>
            )}
          </ReactFlow>
        </div>

        {/* Config panel — right side, conditionally shown */}
        {selectedNode && (
          <div
            className="border-l border-th-border flex flex-col"
            style={{ width: 300, flexShrink: 0, background: 'var(--th-surface)' }}
          >
            <NodeConfigPanel
              node={selectedNode}
              onUpdate={handleConfigUpdate}
              onClose={() => setSelectedNode(null)}
            />
          </div>
        )}
      </div>
    </div>
  )
}
