import { useState, useEffect } from 'react'
import { useTheme, type Theme } from '../../theme'
import { useBranding, type BrandingSettings, type ThemeOverrides } from '../../contexts/BrandingContext'

// All CSS variables defined in index.css, with their light-theme defaults
const CSS_VARS: { name: string; label: string }[] = [
  { name: '--th-bg',              label: 'Background' },
  { name: '--th-surface',         label: 'Surface' },
  { name: '--th-surface-muted',   label: 'Surface Muted' },
  { name: '--th-text',            label: 'Text' },
  { name: '--th-text-secondary',  label: 'Text Secondary' },
  { name: '--th-text-muted',      label: 'Text Muted' },
  { name: '--th-text-subtle',     label: 'Text Subtle' },
  { name: '--th-border',          label: 'Border' },
  { name: '--th-border-subtle',   label: 'Border Subtle' },
  { name: '--th-nav-bg',          label: 'Nav Background' },
  { name: '--th-nav-active',      label: 'Nav Active' },
  { name: '--th-nav-hover',       label: 'Nav Hover' },
  { name: '--th-nav-text',        label: 'Nav Text' },
  { name: '--th-input-bg',        label: 'Input Background' },
  { name: '--th-input-border',    label: 'Input Border' },
  { name: '--th-log-bg',          label: 'Log Background' },
  { name: '--th-log-text',        label: 'Log Text' },
  { name: '--th-progress-track',  label: 'Progress Track' },
]

const THEMES: { id: Theme; label: string; icon: string; colors: { bg: string; surface: string; nav: string; text: string; accent: string } }[] = [
  {
    id: 'light',
    label: 'Light',
    icon: '☀',
    colors: { bg: '#f3f4f6', surface: '#ffffff', nav: '#1f2937', text: '#111827', accent: '#374151' },
  },
  {
    id: 'dark',
    label: 'Dark',
    icon: '●',
    colors: { bg: '#0f172a', surface: '#1e293b', nav: '#020617', text: '#f1f5f9', accent: '#1e293b' },
  },
  {
    id: 'dim',
    label: 'Dim',
    icon: '◑',
    colors: { bg: '#1c2128', surface: '#22272e', nav: '#161b22', text: '#adbac7', accent: '#2d333b' },
  },
]

// Mini preview card to demonstrate custom branding colors
function BrandingPreview({ branding }: { branding: BrandingSettings }) {
  const header = branding.headerBg || '#1f2937'
  const sidebar = branding.sidebarBg || '#374151'
  const primary = branding.primaryColor || '#3b82f6'
  const primaryHover = branding.primaryHoverColor || '#2563eb'
  const title = branding.appTitle || 'EncodeSwarmr'

  return (
    <div className="rounded-lg overflow-hidden border border-th-border shadow-md w-full max-w-sm">
      {/* Header */}
      <div className="flex items-center gap-2 px-3 py-2 text-white text-xs font-semibold" style={{ background: header }}>
        {branding.logoUrl
          ? <img src={branding.logoUrl} alt="Logo" className="h-4 w-4 object-contain" />
          : <span className="w-4 h-4 rounded-sm bg-white/20 flex items-center justify-center text-[8px]">E</span>
        }
        <span>{title}</span>
      </div>
      {/* Body: sidebar + content */}
      <div className="flex" style={{ minHeight: 80 }}>
        <div className="w-20 p-2 text-[9px] text-white/70 flex flex-col gap-1" style={{ background: sidebar }}>
          <div className="bg-white/10 rounded px-1 py-0.5">Dashboard</div>
          <div className="px-1 py-0.5">Jobs</div>
          <div className="px-1 py-0.5">Agents</div>
        </div>
        <div className="flex-1 p-3 bg-th-bg flex flex-col gap-2">
          <div className="text-xs text-th-text font-medium">Preview</div>
          <button
            className="text-[10px] px-2 py-1 rounded text-white self-start transition-colors"
            style={{ background: primary }}
            onMouseEnter={e => (e.currentTarget.style.background = primaryHover)}
            onMouseLeave={e => (e.currentTarget.style.background = primary)}
          >
            Action
          </button>
        </div>
      </div>
    </div>
  )
}

function getCssVarValue(varName: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(varName).trim()
}

export default function ThemeSettings() {
  const { theme, setTheme } = useTheme()
  const { branding, overrides, saveBranding, resetBranding, setOverrides, resetOverrides } = useBranding()

  // Local edit state for branding form
  const [localBranding, setLocalBranding] = useState<BrandingSettings>({ ...branding })

  // Local edit state for CSS overrides
  const [localOverrides, setLocalOverrides] = useState<ThemeOverrides>({ ...overrides })

  // Read current computed values for each var (re-read when theme changes)
  const [computedVars, setComputedVars] = useState<Record<string, string>>({})

  useEffect(() => {
    const vals: Record<string, string> = {}
    CSS_VARS.forEach(v => { vals[v.name] = getCssVarValue(v.name) })
    setComputedVars(vals)
  }, [theme])

  // Keep local branding in sync if external changes occur
  useEffect(() => {
    setLocalBranding({ ...branding })
  }, [branding])

  useEffect(() => {
    setLocalOverrides({ ...overrides })
  }, [overrides])

  const handleBrandingSave = () => {
    saveBranding(localBranding)
  }

  const handleBrandingReset = () => {
    resetBranding()
    setLocalBranding({
      logoUrl: '',
      faviconUrl: '',
      appTitle: '',
      primaryColor: '',
      primaryHoverColor: '',
      sidebarBg: '',
      headerBg: '',
    })
  }

  const handleOverrideSave = () => {
    // Remove empty entries
    const cleaned: ThemeOverrides = {}
    Object.entries(localOverrides).forEach(([k, v]) => {
      if (v.trim()) cleaned[k] = v.trim()
    })
    setOverrides(cleaned)
  }

  const handleOverrideReset = () => {
    resetOverrides()
    setLocalOverrides({})
  }

  const updateOverride = (varName: string, value: string) => {
    setLocalOverrides(prev => ({ ...prev, [varName]: value }))
  }

  const resetSingleOverride = (varName: string) => {
    setLocalOverrides(prev => {
      const next = { ...prev }
      delete next[varName]
      return next
    })
  }

  return (
    <div className="max-w-4xl mx-auto py-6 px-4 sm:px-6 space-y-8">
      <h1 className="text-2xl font-semibold text-th-text">Theme &amp; Branding</h1>

      {/* ── Section 1: Theme Selection ──────────────────────────────────── */}
      <section className="bg-th-surface rounded-lg border border-th-border p-6 space-y-4">
        <h2 className="text-base font-semibold text-th-text">Built-in Themes</h2>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          {THEMES.map(t => (
            <button
              key={t.id}
              onClick={() => setTheme(t.id)}
              className={`rounded-lg border-2 overflow-hidden text-left transition-all focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                theme === t.id ? 'border-blue-500' : 'border-th-border hover:border-th-border'
              }`}
            >
              {/* Mini preview */}
              <div className="h-16 flex" style={{ background: t.colors.bg }}>
                <div className="w-12 flex flex-col gap-0.5 p-1.5" style={{ background: t.colors.nav }}>
                  <div className="h-1 rounded-sm bg-white/30 w-8" />
                  <div className="h-1 rounded-sm bg-white/20 w-6" />
                  <div className="h-1 rounded-sm bg-white/20 w-7" />
                </div>
                <div className="flex-1 p-2 flex flex-col gap-1">
                  <div className="h-1.5 rounded-sm w-3/4" style={{ background: t.colors.text, opacity: 0.7 }} />
                  <div className="h-1.5 rounded-sm w-1/2" style={{ background: t.colors.text, opacity: 0.4 }} />
                  <div className="mt-auto h-3 rounded w-8" style={{ background: t.colors.accent }} />
                </div>
              </div>
              {/* Label */}
              <div className={`px-3 py-2 flex items-center justify-between ${theme === t.id ? 'bg-blue-50 dark:bg-blue-950' : 'bg-th-surface'}`}>
                <span className="text-sm font-medium text-th-text">
                  {t.icon} {t.label}
                </span>
                {theme === t.id && (
                  <span className="text-blue-500 text-xs font-semibold">Active</span>
                )}
              </div>
            </button>
          ))}
        </div>
      </section>

      {/* ── Section 2: Custom Branding Editor ───────────────────────────── */}
      <section className="bg-th-surface rounded-lg border border-th-border p-6 space-y-6">
        <h2 className="text-base font-semibold text-th-text">Custom Branding</h2>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Form */}
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-th-text-secondary mb-1">App Title</label>
              <input
                type="text"
                value={localBranding.appTitle}
                onChange={e => setLocalBranding(prev => ({ ...prev, appTitle: e.target.value }))}
                placeholder="EncodeSwarmr"
                className="w-full rounded border border-th-input-border bg-th-input-bg text-th-text px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-th-text-secondary mb-1">Logo URL</label>
              <input
                type="url"
                value={localBranding.logoUrl}
                onChange={e => setLocalBranding(prev => ({ ...prev, logoUrl: e.target.value }))}
                placeholder="https://example.com/logo.png"
                className="w-full rounded border border-th-input-border bg-th-input-bg text-th-text px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-th-text-secondary mb-1">Favicon URL</label>
              <input
                type="url"
                value={localBranding.faviconUrl}
                onChange={e => setLocalBranding(prev => ({ ...prev, faviconUrl: e.target.value }))}
                placeholder="https://example.com/favicon.ico"
                className="w-full rounded border border-th-input-border bg-th-input-bg text-th-text px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-th-text-secondary mb-1">Primary Color</label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={localBranding.primaryColor || '#3b82f6'}
                    onChange={e => setLocalBranding(prev => ({ ...prev, primaryColor: e.target.value }))}
                    className="h-9 w-14 rounded border border-th-input-border bg-th-input-bg cursor-pointer"
                  />
                  <input
                    type="text"
                    value={localBranding.primaryColor}
                    onChange={e => setLocalBranding(prev => ({ ...prev, primaryColor: e.target.value }))}
                    placeholder="#3b82f6"
                    className="flex-1 rounded border border-th-input-border bg-th-input-bg text-th-text px-2 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
                  />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-th-text-secondary mb-1">Primary Hover</label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={localBranding.primaryHoverColor || '#2563eb'}
                    onChange={e => setLocalBranding(prev => ({ ...prev, primaryHoverColor: e.target.value }))}
                    className="h-9 w-14 rounded border border-th-input-border bg-th-input-bg cursor-pointer"
                  />
                  <input
                    type="text"
                    value={localBranding.primaryHoverColor}
                    onChange={e => setLocalBranding(prev => ({ ...prev, primaryHoverColor: e.target.value }))}
                    placeholder="#2563eb"
                    className="flex-1 rounded border border-th-input-border bg-th-input-bg text-th-text px-2 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
                  />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-th-text-secondary mb-1">Header Background</label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={localBranding.headerBg || '#1f2937'}
                    onChange={e => setLocalBranding(prev => ({ ...prev, headerBg: e.target.value }))}
                    className="h-9 w-14 rounded border border-th-input-border bg-th-input-bg cursor-pointer"
                  />
                  <input
                    type="text"
                    value={localBranding.headerBg}
                    onChange={e => setLocalBranding(prev => ({ ...prev, headerBg: e.target.value }))}
                    placeholder="#1f2937"
                    className="flex-1 rounded border border-th-input-border bg-th-input-bg text-th-text px-2 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
                  />
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-th-text-secondary mb-1">Sidebar Background</label>
                <div className="flex items-center gap-2">
                  <input
                    type="color"
                    value={localBranding.sidebarBg || '#374151'}
                    onChange={e => setLocalBranding(prev => ({ ...prev, sidebarBg: e.target.value }))}
                    className="h-9 w-14 rounded border border-th-input-border bg-th-input-bg cursor-pointer"
                  />
                  <input
                    type="text"
                    value={localBranding.sidebarBg}
                    onChange={e => setLocalBranding(prev => ({ ...prev, sidebarBg: e.target.value }))}
                    placeholder="#374151"
                    className="flex-1 rounded border border-th-input-border bg-th-input-bg text-th-text px-2 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
                  />
                </div>
              </div>
            </div>
          </div>

          {/* Live preview */}
          <div className="flex flex-col gap-3">
            <p className="text-sm font-medium text-th-text-secondary">Live Preview</p>
            <BrandingPreview branding={localBranding} />
          </div>
        </div>

        <div className="flex gap-3 pt-2">
          <button
            onClick={handleBrandingSave}
            className="px-4 py-2 rounded bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium transition-colors"
          >
            Save Branding
          </button>
          <button
            onClick={handleBrandingReset}
            className="px-4 py-2 rounded border border-th-border bg-th-surface hover:bg-th-surface-muted text-th-text-secondary text-sm font-medium transition-colors"
          >
            Reset to Defaults
          </button>
        </div>
      </section>

      {/* ── Section 3: CSS Variable Override Editor ─────────────────────── */}
      <section className="bg-th-surface rounded-lg border border-th-border p-6 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-base font-semibold text-th-text">CSS Variable Overrides</h2>
            <p className="text-xs text-th-text-muted mt-0.5">
              Override any theme variable. Changes apply on top of the selected theme and persist across refreshes.
            </p>
          </div>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-th-border-subtle">
                <th className="text-left px-2 py-2 text-xs font-medium text-th-text-muted w-40">Variable</th>
                <th className="text-left px-2 py-2 text-xs font-medium text-th-text-muted w-12">Theme</th>
                <th className="text-left px-2 py-2 text-xs font-medium text-th-text-muted">Override</th>
                <th className="w-8" />
              </tr>
            </thead>
            <tbody>
              {CSS_VARS.map(v => {
                const computed = computedVars[v.name] || '#000000'
                const override = localOverrides[v.name] ?? ''
                const hasOverride = override !== ''
                return (
                  <tr key={v.name} className="border-b border-th-border-subtle last:border-0">
                    <td className="px-2 py-2">
                      <div className="font-mono text-xs text-th-text-secondary">{v.name}</div>
                      <div className="text-xs text-th-text-muted">{v.label}</div>
                    </td>
                    <td className="px-2 py-2">
                      <div
                        className="w-7 h-7 rounded border border-th-border"
                        style={{ background: computed }}
                        title={computed}
                      />
                    </td>
                    <td className="px-2 py-2">
                      <div className="flex items-center gap-2">
                        <input
                          type="color"
                          value={override || computed}
                          onChange={e => updateOverride(v.name, e.target.value)}
                          className="h-8 w-12 rounded border border-th-input-border bg-th-input-bg cursor-pointer"
                        />
                        <input
                          type="text"
                          value={override}
                          onChange={e => updateOverride(v.name, e.target.value)}
                          placeholder={computed}
                          className={`w-24 rounded border bg-th-input-bg text-th-text px-2 py-1.5 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-blue-500 ${
                            hasOverride ? 'border-blue-400' : 'border-th-input-border'
                          }`}
                        />
                        {hasOverride && (
                          <span className="text-xs text-blue-500 font-medium">overridden</span>
                        )}
                      </div>
                    </td>
                    <td className="px-2 py-2 text-center">
                      {hasOverride && (
                        <button
                          onClick={() => resetSingleOverride(v.name)}
                          title="Clear override"
                          className="text-th-text-muted hover:text-red-500 transition-colors text-xs"
                        >
                          ✕
                        </button>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>

        <div className="flex gap-3 pt-2">
          <button
            onClick={handleOverrideSave}
            className="px-4 py-2 rounded bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium transition-colors"
          >
            Apply Overrides
          </button>
          <button
            onClick={handleOverrideReset}
            className="px-4 py-2 rounded border border-th-border bg-th-surface hover:bg-th-surface-muted text-th-text-secondary text-sm font-medium transition-colors"
          >
            Clear All Overrides
          </button>
        </div>
      </section>
    </div>
  )
}
