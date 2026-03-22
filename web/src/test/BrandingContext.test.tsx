import { render, screen, act } from '@testing-library/react'
import { describe, it, expect, beforeEach } from 'vitest'
import { BrandingProvider, useBranding } from '../contexts/BrandingContext'

function BrandingDisplay() {
  const { branding, setAppTitle } = useBranding()
  return (
    <div>
      <span data-testid="app-title">{branding.appTitle || '(empty)'}</span>
      <span data-testid="logo-url">{branding.logoUrl || '(empty)'}</span>
      <button onClick={() => setAppTitle('Custom Title')}>Set Title</button>
    </div>
  )
}

function BrandingSaveDisplay() {
  const { branding, saveBranding } = useBranding()
  return (
    <div>
      <span data-testid="saved-title">{branding.appTitle || '(empty)'}</span>
      <button
        onClick={() =>
          saveBranding({
            appTitle: 'Persisted Title',
            logoUrl: '',
            faviconUrl: '',
            primaryColor: '',
            primaryHoverColor: '',
            sidebarBg: '',
            headerBg: '',
          })
        }
      >
        Save
      </button>
    </div>
  )
}

describe('BrandingContext', () => {
  beforeEach(() => {
    localStorage.clear()
    // Remove any inline CSS vars set during previous tests
    const el = document.documentElement
    ;[
      '--color-primary', '--color-primary-hover', '--color-sidebar-bg', '--color-header-bg',
    ].forEach(v => el.style.removeProperty(v))
  })

  it('provider renders children', () => {
    render(
      <BrandingProvider>
        <div>Hello World</div>
      </BrandingProvider>
    )
    expect(screen.getByText('Hello World')).toBeInTheDocument()
  })

  it('default values are empty strings', () => {
    render(
      <BrandingProvider>
        <BrandingDisplay />
      </BrandingProvider>
    )
    expect(screen.getByTestId('app-title').textContent).toBe('(empty)')
    expect(screen.getByTestId('logo-url').textContent).toBe('(empty)')
  })

  it('setAppTitle updates context value', async () => {
    render(
      <BrandingProvider>
        <BrandingDisplay />
      </BrandingProvider>
    )
    expect(screen.getByTestId('app-title').textContent).toBe('(empty)')

    await act(async () => {
      screen.getByText('Set Title').click()
    })

    expect(screen.getByTestId('app-title').textContent).toBe('Custom Title')
  })

  it('saveBranding persists to localStorage', async () => {
    render(
      <BrandingProvider>
        <BrandingSaveDisplay />
      </BrandingProvider>
    )

    await act(async () => {
      screen.getByText('Save').click()
    })

    const stored = localStorage.getItem('encodeswarmr-branding')
    expect(stored).not.toBeNull()
    const parsed = JSON.parse(stored!)
    expect(parsed.appTitle).toBe('Persisted Title')
  })

  it('loads persisted branding from localStorage on mount', () => {
    localStorage.setItem(
      'encodeswarmr-branding',
      JSON.stringify({
        appTitle: 'Loaded Title',
        logoUrl: '',
        faviconUrl: '',
        primaryColor: '',
        primaryHoverColor: '',
        sidebarBg: '',
        headerBg: '',
      })
    )

    render(
      <BrandingProvider>
        <BrandingDisplay />
      </BrandingProvider>
    )

    expect(screen.getByTestId('app-title').textContent).toBe('Loaded Title')
  })
})
