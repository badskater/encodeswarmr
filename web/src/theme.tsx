import { createContext, useContext, useState, useEffect, type ReactNode } from 'react'

export type Theme = 'light' | 'dark' | 'dim'

const STORAGE_KEY = 'de-theme'
const OVERRIDES_KEY = 'distencoder-theme-overrides'
const VALID: Theme[] = ['light', 'dark', 'dim']

function getInitialTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY) as Theme | null
  if (stored && VALID.includes(stored)) return stored
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyTheme(theme: Theme) {
  if (theme === 'light') {
    document.documentElement.removeAttribute('data-theme')
  } else {
    document.documentElement.setAttribute('data-theme', theme)
  }
}

// Re-apply any persisted CSS variable overrides so they survive theme switches.
function reapplyOverrides() {
  try {
    const raw = localStorage.getItem(OVERRIDES_KEY)
    if (!raw) return
    const overrides = JSON.parse(raw) as Record<string, string>
    const el = document.documentElement
    Object.entries(overrides).forEach(([key, value]) => {
      el.style.setProperty(key, value)
    })
  } catch {}
}

interface ThemeCtx { theme: Theme; setTheme: (t: Theme) => void }
const ThemeContext = createContext<ThemeCtx>({ theme: 'light', setTheme: () => {} })

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(getInitialTheme)

  useEffect(() => {
    applyTheme(theme)
    localStorage.setItem(STORAGE_KEY, theme)
    // Re-apply overrides after theme switch so custom vars stay on top
    reapplyOverrides()
  }, [theme])

  // Apply on mount to avoid flash before React hydrates, then restore overrides
  useEffect(() => {
    applyTheme(getInitialTheme())
    reapplyOverrides()
  }, [])

  return (
    <ThemeContext.Provider value={{ theme, setTheme: setThemeState }}>
      {children}
    </ThemeContext.Provider>
  )
}

export const useTheme = () => useContext(ThemeContext)
