import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { api, ApiError, type User } from '@/services/api'

interface AuthState {
  user: User | null
  accessToken: string | null
  refreshToken: string | null
  isAuthenticated: boolean

  login: (email: string, password: string) => Promise<void>
  register: (email: string, displayName: string, password: string) => Promise<void>
  logout: () => Promise<void>
  clearAuth: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      accessToken: null,
      refreshToken: null,
      isAuthenticated: false,

      login: async (email, password) => {
        const { user, tokens } = await api.auth.login(email, password)
        set({
          user,
          accessToken: tokens.access_token,
          refreshToken: tokens.refresh_token,
          isAuthenticated: true,
        })
      },

      register: async (email, displayName, password) => {
        const { user, tokens } = await api.auth.register(email, displayName, password)
        set({
          user,
          accessToken: tokens.access_token,
          refreshToken: tokens.refresh_token,
          isAuthenticated: true,
        })
      },

      logout: async () => {
        const { accessToken, refreshToken } = get()
        if (accessToken && refreshToken) {
          try {
            await api.auth.logout(refreshToken, accessToken)
          } catch (e) {
            // Ignore logout errors — clear local state regardless.
            if (!(e instanceof ApiError)) throw e
          }
        }
        get().clearAuth()
      },

      clearAuth: () =>
        set({ user: null, accessToken: null, refreshToken: null, isAuthenticated: false }),
    }),
    {
      name: 'nexustale-auth',
      // Only persist tokens + user — re-derived state is not stored.
      partialize: (s) => ({
        user: s.user,
        accessToken: s.accessToken,
        refreshToken: s.refreshToken,
        isAuthenticated: s.isAuthenticated,
      }),
    },
  ),
)
