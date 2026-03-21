// Node categories
export type NodeCategory =
  | 'input'
  | 'encoding'
  | 'analysis'
  | 'condition'
  | 'audio'
  | 'output'
  | 'notification'
  | 'flow'
  | 'template'

// Base node data attached to every React Flow node
export interface FlowNodeData {
  label: string
  category: NodeCategory
  description?: string
  config: Record<string, unknown>
  // The node type key from the registry (e.g. 'encode_x265')
  nodeType: string
  icon: string
}

// Flow definition as stored in DB
export interface Flow {
  id: string
  name: string
  description: string
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  nodes: any[]
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  edges: any[]
  created_at: string
  updated_at: string
}

// Node template for the sidebar palette
export interface NodeTemplate {
  type: string
  label: string
  category: NodeCategory
  description: string
  icon: string // emoji
  defaultConfig: Record<string, unknown>
  inputs: number  // number of input handles
  outputs: number // 2 for conditionals (true/false)
}
