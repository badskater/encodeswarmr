import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import FlowNode from '../flow/nodes/FlowNode'
import type { NodeProps } from '@xyflow/react'

function makeProps(overrides: Partial<{
  label: string
  category: string
  icon: string
  description: string
  inputs: number
  outputs: number
  nodeType: string
  config: Record<string, unknown>
}> = {}): NodeProps {
  return {
    id: 'test-node',
    selected: false,
    data: {
      label: overrides.label ?? 'Test Node',
      category: overrides.category ?? 'encoding',
      icon: overrides.icon ?? '🎬',
      description: overrides.description ?? 'A test node',
      nodeType: overrides.nodeType ?? 'encode_x265',
      config: overrides.config ?? {},
      inputs: overrides.inputs ?? 1,
      outputs: overrides.outputs ?? 1,
    } as Record<string, unknown>,
  } as NodeProps
}

describe('FlowNode', () => {
  it('renders with the correct label', () => {
    render(<FlowNode {...makeProps({ label: 'x265 Encode' })} />)
    expect(screen.getByText('x265 Encode')).toBeInTheDocument()
  })

  it('renders with the node icon', () => {
    render(<FlowNode {...makeProps({ icon: '🎬' })} />)
    expect(screen.getByText('🎬')).toBeInTheDocument()
  })

  it('has an input handle when inputs > 0', () => {
    render(<FlowNode {...makeProps({ inputs: 1 })} />)
    expect(screen.getByTestId('handle-target')).toBeInTheDocument()
  })

  it('has no input handle when inputs = 0', () => {
    render(<FlowNode {...makeProps({ inputs: 0 })} />)
    expect(screen.queryByTestId('handle-target')).not.toBeInTheDocument()
  })

  it('has 1 output handle for non-condition nodes', () => {
    render(<FlowNode {...makeProps({ category: 'encoding', outputs: 1 })} />)
    const sourceHandles = screen.getAllByTestId('handle-source')
    expect(sourceHandles.length).toBe(1)
  })

  it('has 2 output handles for condition nodes', () => {
    render(<FlowNode {...makeProps({ category: 'condition', outputs: 2 })} />)
    const sourceHandles = screen.getAllByTestId('handle-source')
    expect(sourceHandles.length).toBe(2)
  })

  it('shows checkmark and cross labels for condition node outputs', () => {
    render(<FlowNode {...makeProps({ category: 'condition', outputs: 2 })} />)
    expect(screen.getByText('✓')).toBeInTheDocument()
    expect(screen.getByText('✗')).toBeInTheDocument()
  })

  it('does not show output handle when outputs = 0', () => {
    render(<FlowNode {...makeProps({ outputs: 0 })} />)
    expect(screen.queryByTestId('handle-source')).not.toBeInTheDocument()
  })

  it('shows description when selected', () => {
    const props = makeProps({ description: 'Encode with x265 HEVC' })
    render(<FlowNode {...{ ...props, selected: true }} />)
    expect(screen.getByText('Encode with x265 HEVC')).toBeInTheDocument()
  })

  it('does not show description when not selected', () => {
    const props = makeProps({ description: 'Encode with x265 HEVC' })
    render(<FlowNode {...{ ...props, selected: false }} />)
    expect(screen.queryByText('Encode with x265 HEVC')).not.toBeInTheDocument()
  })
})
