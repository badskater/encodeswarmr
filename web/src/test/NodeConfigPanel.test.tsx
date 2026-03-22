import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import NodeConfigPanel from '../flow/NodeConfigPanel'
import type { Node } from '@xyflow/react'

vi.mock('../api/client', () => ({
  listTemplates: vi.fn().mockResolvedValue([]),
  listWebhooks: vi.fn().mockResolvedValue([]),
}))

function makeNode(nodeType: string, extraConfig: Record<string, unknown> = {}): Node {
  return {
    id: 'test-node-1',
    type: 'flowNode',
    position: { x: 0, y: 0 },
    data: {
      label: nodeType,
      category: 'encoding',
      icon: '🎬',
      description: `${nodeType} node`,
      nodeType,
      config: { ...extraConfig },
      inputs: 1,
      outputs: 1,
    },
  }
}

describe('NodeConfigPanel', () => {
  const onUpdate = vi.fn()
  const onClose = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the node label in the header', () => {
    const node = makeNode('encode_x265')
    render(<NodeConfigPanel node={node} onUpdate={onUpdate} onClose={onClose} />)
    // The node type is used as the label in makeNode — header shows nodeData.label
    expect(screen.getByText('encode_x265')).toBeInTheDocument()
  })

  it('shows CRF slider for x265 node', () => {
    const node = makeNode('encode_x265', { crf: 18, preset: 'slow', profile: 'main10', tune: '', extra_args: '' })
    render(<NodeConfigPanel node={node} onUpdate={onUpdate} onClose={onClose} />)
    expect(screen.getByText(/CRF/)).toBeInTheDocument()
    // range input should be present
    const rangeInput = document.querySelector('input[type="range"]')
    expect(rangeInput).toBeInTheDocument()
  })

  it('shows template dropdown for template_run node', async () => {
    const node = makeNode('template_run', { template_id: '' })
    // update category so panel renders correctly
    node.data.category = 'template'
    render(<NodeConfigPanel node={node} onUpdate={onUpdate} onClose={onClose} />)
    // The "Template" label should appear
    expect(screen.getByText('Template')).toBeInTheDocument()
    const select = document.querySelector('select')
    expect(select).toBeInTheDocument()
  })

  it('shows Apply Changes (save) button', () => {
    const node = makeNode('encode_x265', { crf: 18, preset: 'slow', profile: 'main10', tune: '', extra_args: '' })
    render(<NodeConfigPanel node={node} onUpdate={onUpdate} onClose={onClose} />)
    expect(screen.getByText('Apply Changes')).toBeInTheDocument()
  })

  it('shows close button', () => {
    const node = makeNode('encode_x265')
    render(<NodeConfigPanel node={node} onUpdate={onUpdate} onClose={onClose} />)
    expect(screen.getByTitle('Close')).toBeInTheDocument()
  })

  it('shows no-config message for node types without config fields', () => {
    const node = makeNode('analyze_hdr')
    render(<NodeConfigPanel node={node} onUpdate={onUpdate} onClose={onClose} />)
    expect(screen.getByText('No configuration required.')).toBeInTheDocument()
  })
})
