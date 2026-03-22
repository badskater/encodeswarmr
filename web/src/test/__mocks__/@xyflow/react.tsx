import React from 'react'

// Stub implementations for @xyflow/react to avoid jsdom issues

export const ReactFlow = ({ children }: { children?: React.ReactNode }) => (
  <div data-testid="react-flow">{children}</div>
)

export const Background = () => <div data-testid="rf-background" />
export const BackgroundVariant = { Dots: 'dots', Lines: 'lines', Cross: 'cross' }
export const MiniMap = () => <div data-testid="rf-minimap" />
export const Controls = () => <div data-testid="rf-controls" />
export const Panel = ({ children }: { children?: React.ReactNode }) => (
  <div data-testid="rf-panel">{children}</div>
)
export const Handle = ({ type, position, id, style }: {
  type: string
  position: string
  id?: string
  style?: React.CSSProperties
}) => (
  <div
    data-testid={`handle-${type}`}
    data-handle-id={id}
    data-position={position}
    style={style}
  />
)
export const Position = { Left: 'left', Right: 'right', Top: 'top', Bottom: 'bottom' }

export function useNodesState<T>(initial: T[]) {
  const [nodes, setNodes] = React.useState<T[]>(initial)
  const onNodesChange = () => {}
  return [nodes, setNodes, onNodesChange] as const
}

export function useEdgesState<T>(initial: T[]) {
  const [edges, setEdges] = React.useState<T[]>(initial)
  const onEdgesChange = () => {}
  return [edges, setEdges, onEdgesChange] as const
}

export const addEdge = (params: unknown, edges: unknown[]) => [...edges, params]

export type Node = {
  id: string
  type?: string
  position: { x: number; y: number }
  data: Record<string, unknown>
  selected?: boolean
}

export type Edge = {
  id: string
  source: string
  target: string
}

export type Connection = {
  source: string | null
  target: string | null
  sourceHandle?: string | null
  targetHandle?: string | null
}

export type NodeTypes = Record<string, React.ComponentType<NodeProps>>

export type NodeProps = {
  data: Record<string, unknown>
  selected?: boolean
  id?: string
}

export type OnConnectStartParams = {
  handleId: string | null
  handleType: string | null
  nodeId: string | null
}
