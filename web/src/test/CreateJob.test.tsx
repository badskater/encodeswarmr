import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import CreateJob from '../pages/CreateJob'
import type { Source, Template } from '../types'
import type { Flow } from '../types/flow'

const mockSources: Source[] = [
  {
    id: 'src-1',
    filename: 'movie.mkv',
    path: '\\\\nas\\movie.mkv',
    hdr_type: 'hdr10',
    dv_profile: 0,
    state: 'ready',
    size_bytes: 10000000,
    duration_sec: 3600,
    vmaf_score: null,
    cloud_uri: null,
    created_at: '2024-01-01T00:00:00Z',
  } as Source,
]

const mockTemplates: Template[] = [
  { id: 'tpl-1', name: 'x265 Quality', type: 'bat', extension: 'bat', description: null, content: '', created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
  { id: 'tpl-2', name: 'AviSynth Src', type: 'avs', extension: 'avs', description: null, content: '', created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
]

const mockFlows: Flow[] = [
  { id: 'fl-1', name: 'HDR Flow', description: '', nodes: [], edges: [], created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
]

vi.mock('../api/client', () => ({
  listSources: vi.fn().mockResolvedValue([]),
  listTemplates: vi.fn().mockResolvedValue([]),
  listFlows: vi.fn().mockResolvedValue([]),
  listAnalysisResults: vi.fn().mockResolvedValue([]),
  createJob: vi.fn().mockResolvedValue({ id: 'new-job-id' }),
}))

vi.mock('../components/ChunkBoundaryPreview', () => ({
  default: () => <div data-testid="chunk-preview" />,
}))

function renderCreateJob() {
  return render(
    <MemoryRouter>
      <CreateJob />
    </MemoryRouter>
  )
}

describe('CreateJob', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders source dropdown', async () => {
    renderCreateJob()
    await waitFor(() => {
      expect(screen.getByText('Source *')).toBeInTheDocument()
    })
  })

  it('renders job type dropdown', async () => {
    renderCreateJob()
    await waitFor(() => {
      expect(screen.getByText('Job Type *')).toBeInTheDocument()
    })
  })

  it('Use Flow Pipeline toggle exists', async () => {
    renderCreateJob()
    await waitFor(() => {
      expect(screen.getByText('Use Flow Pipeline')).toBeInTheDocument()
      const toggle = screen.getByRole('switch')
      expect(toggle).toBeInTheDocument()
      expect(toggle).toHaveAttribute('aria-checked', 'false')
    })
  })

  it('shows flow dropdown when flow toggle is enabled', async () => {
    const { listFlows } = await import('../api/client')
    vi.mocked(listFlows).mockResolvedValueOnce(mockFlows)

    renderCreateJob()
    await waitFor(() => {
      expect(screen.getByRole('switch')).toBeInTheDocument()
    })

    const toggle = screen.getByRole('switch')
    fireEvent.click(toggle)

    await waitFor(() => {
      expect(screen.getByText('Flow *')).toBeInTheDocument()
    })
  })

  it('shows template dropdowns when flow toggle is disabled', async () => {
    const { listTemplates } = await import('../api/client')
    vi.mocked(listTemplates).mockResolvedValueOnce(mockTemplates)

    renderCreateJob()
    await waitFor(() => {
      expect(screen.getByText('Run Script Template')).toBeInTheDocument()
    })
  })

  it('Chunked Encoding checkbox hides and shows chunk config', async () => {
    const { listSources, listTemplates } = await import('../api/client')
    vi.mocked(listSources).mockResolvedValueOnce(mockSources)
    vi.mocked(listTemplates).mockResolvedValueOnce(mockTemplates)

    renderCreateJob()
    await waitFor(() => {
      expect(screen.getByText('Chunked Encoding')).toBeInTheDocument()
    })

    // Chunk size input not visible before enabling
    expect(screen.queryByText('Chunk Size (frames)')).not.toBeInTheDocument()

    const checkbox = screen.getByLabelText('Chunked Encoding')
    fireEvent.click(checkbox)

    await waitFor(() => {
      expect(screen.getByText('Chunk Size (frames)')).toBeInTheDocument()
    })
  })

  it('renders Create Job submit button', async () => {
    renderCreateJob()
    await waitFor(() => {
      expect(screen.getByText('Create Job')).toBeInTheDocument()
    })
  })
})
