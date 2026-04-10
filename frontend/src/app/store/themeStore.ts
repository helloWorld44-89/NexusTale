import { create } from 'zustand'

type Theme = 'dark' | 'light'

const STORAGE_KEY = 'nexustale-theme'

function applyTheme(theme: Theme) {
  const root = document.documentElement
  root.classList.remove('dark', 'light')
  root.classList.add(theme)
  localStorage.setItem(STORAGE_KEY, theme)
}

function getInitialTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY) as Theme | null
  if (stored === 'dark' || stored === 'light') return stored
  // Fall back to system preference
  return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

interface ThemeState {
  theme: Theme
  setTheme: (theme: Theme) => void
  toggleTheme: () => void
  init: () => void
}

export const useThemeStore = create<ThemeState>((set, get) => ({
  theme: 'dark',

  init: () => {
    const theme = getInitialTheme()
    applyTheme(theme)
    set({ theme })
  },

  setTheme: (theme) => {
    applyTheme(theme)
    set({ theme })
  },

  toggleTheme: () => {
    const next: Theme = get().theme === 'dark' ? 'light' : 'dark'
    applyTheme(next)
    set({ theme: next })
  },
}))
