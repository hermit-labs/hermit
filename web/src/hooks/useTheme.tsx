import { createContext, useContext, useState, useEffect, useCallback } from 'react'
import type { ReactNode } from 'react'

type ThemeMode = 'system' | 'light' | 'dark'

interface ThemeContextValue {
  mode: ThemeMode
  isDark: boolean
  setMode: (mode: ThemeMode) => void
}

const STORAGE_KEY = 'hermit-theme'

function getSystemDark() {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function resolveDark(mode: ThemeMode): boolean {
  if (mode === 'system') return getSystemDark()
  return mode === 'dark'
}

function applyTheme(dark: boolean, animate = false) {
  const el = document.documentElement
  if (animate) {
    el.classList.add('theme-transition')
    setTimeout(() => el.classList.remove('theme-transition'), 350)
  }
  el.setAttribute('data-theme', dark ? 'hermit-dark' : 'hermit-light')
}

const ThemeContext = createContext<ThemeContextValue | null>(null)

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<ThemeMode>(() => {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'light' || saved === 'dark' || saved === 'system') return saved
    return 'system'
  })
  const [isDark, setIsDark] = useState(() => resolveDark(mode))
  const [animate, setAnimate] = useState(false)

  useEffect(() => {
    const dark = resolveDark(mode)
    setIsDark(dark)
    applyTheme(dark, animate)
    localStorage.setItem(STORAGE_KEY, mode)
    if (animate) setAnimate(false)
  }, [mode, animate])

  useEffect(() => {
    if (mode !== 'system') return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => {
      setIsDark(mq.matches)
      applyTheme(mq.matches, true)
    }
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [mode])

  const setMode = useCallback((m: ThemeMode) => {
    setAnimate(true)
    setModeState(m)
  }, [])

  return (
    <ThemeContext value={{ mode, isDark, setMode }}>
      {children}
    </ThemeContext>
  )
}

export function useTheme() {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme must be used within ThemeProvider')
  return ctx
}
