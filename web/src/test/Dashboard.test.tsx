import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Dashboard from '../pages/Dashboard'

vi.mock('../api/client', () => ({
  listJobs: vi.fn().mockResolvedValue([]),
  listAgents: vi.fn().mockResolvedValue([]),
  getThroughput: vi.fn().mockResolvedValue([]),
  getQueueSummary: vi.fn().mockResolvedValue({ pending: 0, running: 0, estimated_completion_sec: null }),
  getRecentActivity: vi.fn().mockResolvedValue([]),
  listAgentMetrics: vi.fn().mockResolvedValue([]),
}))

vi.mock('../hooks/useAutoRefresh', () => ({
  useAutoRefresh: vi.fn(),
}))

function renderDashboard() {
  return render(
    <MemoryRouter>
      <Dashboard />
    </MemoryRouter>
  )
}

describe('Dashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders without crashing and shows heading', async () => {
    renderDashboard()
    await waitFor(() => {
      expect(screen.getByText('Dashboard')).toBeInTheDocument()
    })
  })

  it('shows Queue Depth section', async () => {
    renderDashboard()
    await waitFor(() => {
      expect(screen.getByText('Queue Depth')).toBeInTheDocument()
    })
  })

  it('shows Recent Activity section', async () => {
    renderDashboard()
    await waitFor(() => {
      expect(screen.getByText('Recent Activity')).toBeInTheDocument()
    })
  })

  it('shows stat cards', async () => {
    renderDashboard()
    await waitFor(() => {
      expect(screen.getByText('Total Jobs')).toBeInTheDocument()
      expect(screen.getByText('Running Jobs')).toBeInTheDocument()
      expect(screen.getByText('Idle Agents')).toBeInTheDocument()
      expect(screen.getByText('Offline Agents')).toBeInTheDocument()
    })
  })

  it('handles API errors gracefully', async () => {
    const { listJobs } = await import('../api/client')
    vi.mocked(listJobs).mockRejectedValueOnce(new Error('Network error'))

    renderDashboard()
    await waitFor(() => {
      expect(screen.getByText('Network error')).toBeInTheDocument()
    })
  })
})
