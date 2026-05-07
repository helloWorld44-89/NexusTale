import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { api, ApiError, type User } from '@/services/api'

// Decode the JWT payload to get the `exp` claim (seconds since epoch).
// Safe for our own tokens — JWTs are signed, not encrypted.
function parseJwtExpMs(token: string): number | null {
  try {
    const b64 = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    const payload = JSON.parse(atob(b64))
    return typeof payload.exp === 'number' ? payload.exp * 1000 : null
  } catch {
    return null
  }
}

// Refresh the access token this many ms before it expires.
const REFRESH_AHEAD_MS = 5 * 60 * 1000

// Module-level timer — one active at a time, cleared on logout/clearAuth.
let _refreshTimer: ReturnType<typeof setTimeout> | null = null

interface AuthState {
  user: User | null
  accessToken: string | null
  refreshToken: string | null
  isAuthenticated: boolean

  login:           (email: string, password: string) => Promise<void>
  register:        (email: string, displayName: string, password: string) => Promise<void>
  logout:          () => Promise<void>
  clearAuth:       () => void
  scheduleRefresh: () => void
  silentRefresh:   () => Promise<void>
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      accessToken: null,
      refreshToken: null,
      isAuthenticated: false,

      scheduleRefresh: () => {
        const { accessToken } = get()
        if (!accessToken) return
        if (_refreshTimer) clearTimeout(_refreshTimer)
        const exp = parseJwtExpMs(accessToken)
        if (!exp) return
        const delay = Math.max(0, exp - Date.now() - REFRESH_AHEAD_MS)
        _refreshTimer = setTimeout(() => get().silentRefresh(), delay)
      },

      silentRefresh: async () => {
        const { refreshToken } = get()
        if (!refreshToken) return
        try {
          const { access_token, refresh_token } = await api.auth.refresh(refreshToken)
          set({ accessToken: access_token, refreshToken: refresh_token })
          get().scheduleRefresh()
        } catch {
          get().clearAuth()
        }
      },

      login: async (email, password) => {
        const { user, tokens } = await api.auth.login(email, password)
        set({ user, accessToken: tokens.access_token, refreshToken: tokens.refresh_token, isAuthenticated: true })
        get().scheduleRefresh()
      },

      register: async (email, displayName, password) => {
        const { user, tokens } = await api.auth.register(email, displayName, password)
        set({ user, accessToken: tokens.access_token, refreshToken: tokens.refresh_token, isAuthenticated: true })
        get().scheduleRefresh()
      },

      logout: async () => {
        if (_refreshTimer) { clearTimeout(_refreshTimer); _refreshTimer = null }
        const { accessToken, refreshToken } = get()
        if (accessToken && refreshToken) {
          try {
            await api.auth.logout(refreshToken, accessToken)
          } catch (e) {
            if (!(e instanceof ApiError)) throw e
          }
        }
        get().clearAuth()
      },

      clearAuth: () => {
        if (_refreshTimer) { clearTimeout(_refreshTimer); _refreshTimer = null }
        set({ user: null, accessToken: null, refreshToken: null, isAuthenticated: false })
      },
    }),
    {
      name: 'nexustale-auth',
      partialize: (s) => ({
        user:            s.user,
        accessToken:     s.accessToken,
        refreshToken:    s.refreshToken,
        isAuthenticated: s.isAuthenticated,
      }),
      onRehydrateStorage: () => (state) => {
        // On page load, schedule a proactive refresh for the restored token.
        // If the token is already expired or near expiry, this fires immediately.
        if (state?.isAuthenticated && state.accessToken) {
          setTimeout(() => state.scheduleRefresh(), 0)
        }
      },
    },
  ),
)
