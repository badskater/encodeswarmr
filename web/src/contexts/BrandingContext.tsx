import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  type ReactNode,
} from 'react'

const BRANDING_KEY = 'distencoder-branding'
const OVERRIDES_KEY = 'distencoder-theme-overrides'

export interface BrandingSettings {
  logoUrl: string
  faviconUrl: string
  appTitle: string
  primaryColor: string
  primaryHoverColor: string
  sidebarBg: string
  headerBg: string
}

const DEFAULT_BRANDING: BrandingSettings = {
  logoUrl: '',
  faviconUrl: '',
  appTitle: '',
  primaryColor: '',
  primaryHoverColor: '',
  sidebarBg: '',
  headerBg: '',
}

export type ThemeOverrides = Record<string, string>

interface BrandingCtx {
  branding: BrandingSettings
  overrides: ThemeOverrides
  setLogoUrl: (v: string) => void
  setFaviconUrl: (v: string) => void
  setAppTitle: (v: string) => void
  setCustomColor: (key: keyof BrandingSettings, value: string) => void
  saveBranding: (b: BrandingSettings) => void
  resetBranding: () => void
  setOverrides: (o: ThemeOverrides) => void
  resetOverrides: () => void
}

const BrandingContext = createContext<BrandingCtx>({
  branding: DEFAULT_BRANDING,
  overrides: {},
  setLogoUrl: () => {},
  setFaviconUrl: () => {},
  setAppTitle: () => {},
  setCustomColor: () => {},
  saveBranding: () => {},
  resetBranding: () => {},
  setOverrides: () => {},
  resetOverrides: () => {},
})

function loadBranding(): BrandingSettings {
  try {
    const raw = localStorage.getItem(BRANDING_KEY)
    if (raw) return { ...DEFAULT_BRANDING, ...JSON.parse(raw) }
  } catch {}
  return { ...DEFAULT_BRANDING }
}

function loadOverrides(): ThemeOverrides {
  try {
    const raw = localStorage.getItem(OVERRIDES_KEY)
    if (raw) return JSON.parse(raw) as ThemeOverrides
  } catch {}
  return {}
}

function applyOverrides(overrides: ThemeOverrides) {
  const el = document.documentElement
  Object.entries(overrides).forEach(([key, value]) => {
    el.style.setProperty(key, value)
  })
}

function applyBrandingColors(b: BrandingSettings) {
  const el = document.documentElement
  if (b.primaryColor)      el.style.setProperty('--color-primary', b.primaryColor)
  else                      el.style.removeProperty('--color-primary')
  if (b.primaryHoverColor) el.style.setProperty('--color-primary-hover', b.primaryHoverColor)
  else                      el.style.removeProperty('--color-primary-hover')
  if (b.sidebarBg)         el.style.setProperty('--color-sidebar-bg', b.sidebarBg)
  else                      el.style.removeProperty('--color-sidebar-bg')
  if (b.headerBg)          el.style.setProperty('--color-header-bg', b.headerBg)
  else                      el.style.removeProperty('--color-header-bg')

  // Update favicon if set
  if (b.faviconUrl) {
    const existing = document.querySelector<HTMLLinkElement>('link[rel~="icon"]')
    if (existing) {
      existing.href = b.faviconUrl
    } else {
      const link = document.createElement('link')
      link.rel = 'icon'
      link.href = b.faviconUrl
      document.head.appendChild(link)
    }
  }
}

export function BrandingProvider({ children }: { children: ReactNode }) {
  const [branding, setBrandingState] = useState<BrandingSettings>(loadBranding)
  const [overrides, setOverridesState] = useState<ThemeOverrides>(loadOverrides)

  // Apply on mount
  useEffect(() => {
    applyBrandingColors(branding)
    applyOverrides(overrides)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const saveBranding = useCallback((b: BrandingSettings) => {
    setBrandingState(b)
    localStorage.setItem(BRANDING_KEY, JSON.stringify(b))
    applyBrandingColors(b)
  }, [])

  const resetBranding = useCallback(() => {
    setBrandingState({ ...DEFAULT_BRANDING })
    localStorage.removeItem(BRANDING_KEY)
    applyBrandingColors({ ...DEFAULT_BRANDING })
  }, [])

  const setLogoUrl = useCallback((v: string) => {
    setBrandingState(prev => { const next = { ...prev, logoUrl: v }; return next })
  }, [])

  const setFaviconUrl = useCallback((v: string) => {
    setBrandingState(prev => ({ ...prev, faviconUrl: v }))
  }, [])

  const setAppTitle = useCallback((v: string) => {
    setBrandingState(prev => ({ ...prev, appTitle: v }))
  }, [])

  const setCustomColor = useCallback((key: keyof BrandingSettings, value: string) => {
    setBrandingState(prev => ({ ...prev, [key]: value }))
  }, [])

  const setOverrides = useCallback((o: ThemeOverrides) => {
    setOverridesState(o)
    localStorage.setItem(OVERRIDES_KEY, JSON.stringify(o))
    applyOverrides(o)
  }, [])

  const resetOverrides = useCallback(() => {
    setOverridesState({})
    localStorage.removeItem(OVERRIDES_KEY)
    // Clear all inline CSS vars that were set
    const el = document.documentElement
    Object.keys(overrides).forEach(key => el.style.removeProperty(key))
  }, [overrides])

  return (
    <BrandingContext.Provider value={{
      branding,
      overrides,
      setLogoUrl,
      setFaviconUrl,
      setAppTitle,
      setCustomColor,
      saveBranding,
      resetBranding,
      setOverrides,
      resetOverrides,
    }}>
      {children}
    </BrandingContext.Provider>
  )
}

export const useBranding = () => useContext(BrandingContext)
