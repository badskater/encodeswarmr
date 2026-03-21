// ThemeContext re-exports the ThemeProvider and useTheme hook from theme.tsx so
// that consumers can import from a conventional contexts/ path.
//
// Usage:
//   import { ThemeProvider, useTheme } from '../contexts/ThemeContext'
export { ThemeProvider, useTheme } from '../theme'
export type { Theme } from '../theme'
