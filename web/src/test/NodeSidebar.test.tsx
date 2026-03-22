import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import NodeSidebar from '../flow/NodeSidebar'
import { CATEGORY_LABELS, CATEGORY_ORDER, NODE_BY_CATEGORY } from '../flow/nodeRegistry'

describe('NodeSidebar', () => {
  it('renders all 9 categories', () => {
    render(<NodeSidebar />)
    for (const cat of CATEGORY_ORDER) {
      const label = CATEGORY_LABELS[cat]
      // Category labels are rendered as-is in the DOM; CSS uppercase is visual only
      const elements = screen.getAllByText(label)
      expect(elements.length).toBeGreaterThan(0)
    }
  })

  it('renders a search input', () => {
    render(<NodeSidebar />)
    expect(screen.getByPlaceholderText('Search nodes…')).toBeInTheDocument()
  })

  it('shows the Node Palette header', () => {
    render(<NodeSidebar />)
    expect(screen.getByText('Node Palette')).toBeInTheDocument()
  })

  it('shows drag hint footer', () => {
    render(<NodeSidebar />)
    expect(screen.getByText('Drag nodes onto canvas')).toBeInTheDocument()
  })

  it('search filters nodes by name', () => {
    render(<NodeSidebar />)
    const input = screen.getByPlaceholderText('Search nodes…')
    fireEvent.change(input, { target: { value: 'x265' } })

    // x265 Encode should appear in search results
    expect(screen.getByText('x265 Encode')).toBeInTheDocument()
    // Audio nodes should not appear
    expect(screen.queryByText('Convert to FLAC')).not.toBeInTheDocument()
  })

  it('search is case-insensitive', () => {
    render(<NodeSidebar />)
    const input = screen.getByPlaceholderText('Search nodes…')
    fireEvent.change(input, { target: { value: 'ENCODE' } })

    expect(screen.getByText('x265 Encode')).toBeInTheDocument()
  })

  it('shows all nodes when search is cleared', () => {
    render(<NodeSidebar />)
    const input = screen.getByPlaceholderText('Search nodes…')
    fireEvent.change(input, { target: { value: 'x265' } })
    fireEvent.change(input, { target: { value: '' } })

    // Category headers should reappear (DOM text is not uppercased; CSS handles that)
    expect(screen.getAllByText('Input').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Encoding').length).toBeGreaterThan(0)
  })

  it('each category shows node count badge', () => {
    render(<NodeSidebar />)
    for (const cat of CATEGORY_ORDER) {
      const nodeCount = (NODE_BY_CATEGORY.get(cat) ?? []).length
      // The count badge values should exist in the document
      expect(screen.getAllByText(String(nodeCount)).length).toBeGreaterThan(0)
    }
  })

  it('node cards have the draggable attribute', () => {
    render(<NodeSidebar />)
    // Expand the first category (input — open by default at idx 0)
    // Node cards that are visible should be draggable
    const draggableElements = document.querySelectorAll('[draggable="true"]')
    expect(draggableElements.length).toBeGreaterThan(0)
  })
})
