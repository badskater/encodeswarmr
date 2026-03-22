import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Templates from '../pages/admin/Templates'
import type { Template } from '../types'

const mockTemplates: Template[] = [
  { id: 'tpl-1', name: 'x265 Quality Encode', type: 'bat', extension: 'bat', description: 'High quality', content: '@echo off\n...', created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z' },
  { id: 'tpl-2', name: 'AviSynth Source', type: 'avs', extension: 'avs', description: null, content: 'FFVideoSource()', created_at: '2024-01-02T00:00:00Z', updated_at: '2024-01-02T00:00:00Z' },
]

vi.mock('../../api/client', () => ({
  listTemplates: vi.fn().mockResolvedValue([]),
  createTemplate: vi.fn().mockResolvedValue({ id: 'new-tpl' }),
  updateTemplate: vi.fn().mockResolvedValue(undefined),
  deleteTemplate: vi.fn().mockResolvedValue(undefined),
}))

describe('Templates', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders Templates heading', async () => {
    render(<Templates />)
    await waitFor(() => {
      expect(screen.getByText('Templates')).toBeInTheDocument()
    })
  })

  it('renders empty state when no templates', async () => {
    render(<Templates />)
    await waitFor(() => {
      expect(screen.getByText('No templates')).toBeInTheDocument()
    })
  })

  it('renders template list with names', async () => {
    const { listTemplates } = await import('../../api/client')
    vi.mocked(listTemplates).mockResolvedValueOnce(mockTemplates)

    render(<Templates />)
    await waitFor(() => {
      expect(screen.getByText('x265 Quality Encode')).toBeInTheDocument()
      expect(screen.getByText('AviSynth Source')).toBeInTheDocument()
    })
  })

  it('Create from Starter button exists', async () => {
    render(<Templates />)
    await waitFor(() => {
      expect(screen.getByText('Create from Starter')).toBeInTheDocument()
    })
  })

  it('clicking Create from Starter shows starter grid with 8 cards', async () => {
    render(<Templates />)
    await waitFor(() => {
      expect(screen.getByText('Create from Starter')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Create from Starter'))

    await waitFor(() => {
      expect(screen.getByText('Choose a Starter Template')).toBeInTheDocument()
      // 8 starter templates — verify at least a few by their specific names
      expect(screen.getByText('x265 Quality Encode')).toBeInTheDocument()
      expect(screen.getByText('x265 HDR10 Passthrough')).toBeInTheDocument()
      expect(screen.getByText('x265 Dolby Vision')).toBeInTheDocument()
      expect(screen.getByText('SVT-AV1 Encode')).toBeInTheDocument()
      expect(screen.getByText('AviSynth Source')).toBeInTheDocument()
      expect(screen.getByText('VapourSynth Source')).toBeInTheDocument()
      // Count all "Use" buttons (one per starter card)
      const useButtons = screen.getAllByText('Use')
      expect(useButtons.length).toBe(8)
    })
  })

  it('Variable Reference panel toggles on click', async () => {
    render(<Templates />)
    await waitFor(() => {
      expect(screen.getByText('Add Template')).toBeInTheDocument()
    })

    // Open the new template form first (to show Variable Reference)
    fireEvent.click(screen.getByText('Add Template'))

    await waitFor(() => {
      expect(screen.getByText('Variable Reference')).toBeInTheDocument()
    })

    // Initially collapsed — clicking should expand it
    fireEvent.click(screen.getByText('Variable Reference'))
    await waitFor(() => {
      expect(screen.getByText('Built-in Variables')).toBeInTheDocument()
    })
  })
})
