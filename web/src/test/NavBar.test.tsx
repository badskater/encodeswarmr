import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import NavBar from '../components/NavBar'
import { BrandingProvider } from '../contexts/BrandingContext'

// Mock api/client to prevent fetch calls
vi.mock('../api/client', () => ({
  logout: vi.fn().mockResolvedValue(undefined),
}))

// Mock ThemePicker since it depends on ThemeContext
vi.mock('../components/ThemePicker', () => ({
  default: () => <div data-testid="theme-picker" />,
}))

function renderNavBar(role = 'user', onLogout = vi.fn()) {
  return render(
    <BrandingProvider>
      <MemoryRouter>
        <NavBar role={role} onLogout={onLogout} />
      </MemoryRouter>
    </BrandingProvider>
  )
}

describe('NavBar', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('renders the default app title when no branding is set', () => {
    renderNavBar()
    expect(screen.getByText('Encoder')).toBeInTheDocument()
  })

  it('shows custom branding title when set in localStorage', () => {
    localStorage.setItem(
      'encodeswarmr-branding',
      JSON.stringify({ appTitle: 'MyEncoder', logoUrl: '', faviconUrl: '', primaryColor: '', primaryHoverColor: '', sidebarBg: '', headerBg: '' })
    )
    renderNavBar()
    expect(screen.getByText('MyEncoder')).toBeInTheDocument()
  })

  it('renders main navigation links', () => {
    renderNavBar()
    expect(screen.getAllByText('Dashboard').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Sources').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Jobs').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Agents').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Flows').length).toBeGreaterThan(0)
  })

  it('does not show admin links when role is user', () => {
    renderNavBar('user')
    expect(screen.queryByText('Templates')).not.toBeInTheDocument()
    expect(screen.queryByText('Users')).not.toBeInTheDocument()
  })

  it('shows admin links when role is admin', () => {
    renderNavBar('admin')
    expect(screen.getAllByText('Templates').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Users').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Webhooks').length).toBeGreaterThan(0)
  })

  it('hamburger button is present and toggles mobile menu', () => {
    renderNavBar()
    const hamburger = screen.getByLabelText('Toggle navigation menu')
    expect(hamburger).toBeInTheDocument()
    expect(hamburger).toHaveAttribute('aria-expanded', 'false')

    fireEvent.click(hamburger)
    expect(hamburger).toHaveAttribute('aria-expanded', 'true')

    fireEvent.click(hamburger)
    expect(hamburger).toHaveAttribute('aria-expanded', 'false')
  })
})
