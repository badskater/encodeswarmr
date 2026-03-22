import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import FlowEditor from '../pages/FlowEditor'

vi.mock('../api/client', () => ({
  getFlow: vi.fn(),
  createFlow: vi.fn().mockResolvedValue({ id: 'new-flow-id' }),
  updateFlow: vi.fn().mockResolvedValue(undefined),
  deleteFlow: vi.fn().mockResolvedValue(undefined),
}))

function renderFlowEditor(id?: string) {
  const path = id ? `/flows/editor/${id}` : '/flows/editor'
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/flows/editor" element={<FlowEditor />} />
        <Route path="/flows/editor/:id" element={<FlowEditor />} />
        <Route path="/flows" element={<div>Flows List</div>} />
      </Routes>
    </MemoryRouter>
  )
}

describe('FlowEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders the flow editor toolbar with Save Flow button', async () => {
    renderFlowEditor()
    await waitFor(() => {
      expect(screen.getByText('Save Flow')).toBeInTheDocument()
    })
  })

  it('renders the flow name input', async () => {
    renderFlowEditor()
    await waitFor(() => {
      const input = screen.getByPlaceholderText('Flow name')
      expect(input).toBeInTheDocument()
    })
  })

  it('flow name input has default value', async () => {
    renderFlowEditor()
    await waitFor(() => {
      const input = screen.getByPlaceholderText('Flow name') as HTMLInputElement
      expect(input.value).toBe('Untitled Flow')
    })
  })

  it('renders the back-to-flows navigation link', async () => {
    renderFlowEditor()
    await waitFor(() => {
      expect(screen.getByText('← Flows')).toBeInTheDocument()
    })
  })

  it('renders the node sidebar', async () => {
    renderFlowEditor()
    await waitFor(() => {
      expect(screen.getByText('Node Palette')).toBeInTheDocument()
    })
  })

  it('renders the ReactFlow canvas area', async () => {
    renderFlowEditor()
    await waitFor(() => {
      expect(screen.getByTestId('react-flow')).toBeInTheDocument()
    })
  })

  it('shows Undo and Redo buttons', async () => {
    renderFlowEditor()
    await waitFor(() => {
      expect(screen.getByTitle('Undo (Ctrl+Z)')).toBeInTheDocument()
      expect(screen.getByTitle('Redo (Ctrl+Y)')).toBeInTheDocument()
    })
  })

  it('does not show Delete button for new flows', async () => {
    renderFlowEditor()
    await waitFor(() => {
      // Verify Save button is there but no Delete
      expect(screen.getByText('Save Flow')).toBeInTheDocument()
    })
    // Wait a tick for loading to settle
    expect(screen.queryByText('Delete')).not.toBeInTheDocument()
  })
})
