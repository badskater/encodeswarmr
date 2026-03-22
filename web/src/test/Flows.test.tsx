import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Flows from '../pages/Flows'
import type { Flow } from '../types/flow'

const mockFlows: Flow[] = [
  {
    id: 'flow-1',
    name: 'My HDR Flow',
    description: 'HDR encoding pipeline',
    nodes: [{} as never, {} as never, {} as never],
    edges: [{} as never],
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-02T00:00:00Z',
  },
  {
    id: 'flow-2',
    name: 'AV1 Batch Flow',
    description: '',
    nodes: [{} as never],
    edges: [],
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-03T00:00:00Z',
  },
]

vi.mock('../api/client', () => ({
  listFlows: vi.fn().mockResolvedValue([]),
  deleteFlow: vi.fn().mockResolvedValue(undefined),
  createFlow: vi.fn(),
}))

function renderFlows() {
  return render(
    <MemoryRouter>
      <Flows />
    </MemoryRouter>
  )
}

describe('Flows', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders empty state when no flows exist', async () => {
    renderFlows()
    await waitFor(() => {
      expect(screen.getByText('No flows yet')).toBeInTheDocument()
    })
  })

  it('renders flow list with names and node counts', async () => {
    const { listFlows } = await import('../api/client')
    vi.mocked(listFlows).mockResolvedValueOnce(mockFlows)

    renderFlows()
    await waitFor(() => {
      expect(screen.getByText('My HDR Flow')).toBeInTheDocument()
      expect(screen.getByText('AV1 Batch Flow')).toBeInTheDocument()
    })

    // Node count badges — use getAllByText since '1' may appear in multiple badges
    expect(screen.getByText('3')).toBeInTheDocument() // 3 nodes in flow-1
    expect(screen.getAllByText('1').length).toBeGreaterThan(0) // 1 node in flow-2 (also 1 edge in flow-1)
  })

  it('Create New Flow button exists', async () => {
    renderFlows()
    await waitFor(() => {
      expect(screen.getByText('Create New Flow')).toBeInTheDocument()
    })
  })

  it('shows Delete buttons when flows are present', async () => {
    const { listFlows } = await import('../api/client')
    vi.mocked(listFlows).mockResolvedValueOnce(mockFlows)

    renderFlows()
    await waitFor(() => {
      const deleteButtons = screen.getAllByText('Delete')
      expect(deleteButtons.length).toBe(2)
    })
  })
})
