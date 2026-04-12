import '@testing-library/jest-dom'
import { vi } from 'vitest'

// Mock fetch globally
;(globalThis as Record<string, unknown>).fetch = vi.fn()

// Mock window.matchMedia (used by theme.tsx)
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})
