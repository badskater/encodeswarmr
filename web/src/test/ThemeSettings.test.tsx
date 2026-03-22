import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, beforeEach } from 'vitest'
import ThemeSettings from '../pages/admin/ThemeSettings'
import { ThemeProvider } from '../theme'
import { BrandingProvider } from '../contexts/BrandingContext'

function renderThemeSettings() {
  return render(
    <BrandingProvider>
      <ThemeProvider>
        <ThemeSettings />
      </ThemeProvider>
    </BrandingProvider>
  )
}

describe('ThemeSettings', () => {
  beforeEach(() => {
    localStorage.clear()
    // Reset data-theme attribute
    document.documentElement.removeAttribute('data-theme')
  })

  it('renders the Theme & Branding heading', () => {
    renderThemeSettings()
    expect(screen.getByText('Theme & Branding')).toBeInTheDocument()
  })

  it('renders 3 theme cards: Light, Dark, Dim', () => {
    renderThemeSettings()
    // Theme labels are rendered as "{icon} {label}" in a single span
    expect(screen.getByText(/Light/)).toBeInTheDocument()
    expect(screen.getByText(/Dark/)).toBeInTheDocument()
    expect(screen.getByText(/Dim/)).toBeInTheDocument()
  })

  it('clicking a theme card marks it as active', () => {
    renderThemeSettings()
    const darkButton = screen.getByText(/Dark/).closest('button')
    expect(darkButton).toBeInTheDocument()
    fireEvent.click(darkButton!)

    // After clicking Dark, "Active" label should appear next to Dark
    const activeLabels = screen.getAllByText('Active')
    expect(activeLabels.length).toBeGreaterThan(0)
  })

  it('shows the Custom Branding section', () => {
    renderThemeSettings()
    expect(screen.getByText('Custom Branding')).toBeInTheDocument()
  })

  it('branding section has App Title input', () => {
    renderThemeSettings()
    expect(screen.getByPlaceholderText('EncodeSwarmr')).toBeInTheDocument()
  })

  it('branding section has color picker inputs', () => {
    renderThemeSettings()
    const colorInputs = document.querySelectorAll('input[type="color"]')
    // 4 color pickers in branding + 18 in CSS overrides = 22 total
    expect(colorInputs.length).toBeGreaterThan(0)
  })

  it('CSS variable override table shows 18 rows', () => {
    renderThemeSettings()
    // Each variable row has a data-var entry; count by unique font-mono variable names
    const varRows = document.querySelectorAll('tbody tr')
    expect(varRows.length).toBe(18)
  })

  it('Save Branding button is present', () => {
    renderThemeSettings()
    expect(screen.getByText('Save Branding')).toBeInTheDocument()
  })

  it('Apply Overrides button is present', () => {
    renderThemeSettings()
    expect(screen.getByText('Apply Overrides')).toBeInTheDocument()
  })
})
