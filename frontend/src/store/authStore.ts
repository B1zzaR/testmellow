import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AuthUser {
  id: string
  is_admin: boolean
  email: string | null
}

interface AuthState {
  token: string | null
  user: AuthUser | null
  setAuth: (token: string, user: AuthUser) => void
  clearAuth: () => void
  isAuthenticated: () => boolean
  isAdmin: () => boolean
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      user: null,

      setAuth: (token, user) => {
        localStorage.setItem('auth_token', token)
        set({ token, user })
      },

      clearAuth: () => {
        localStorage.removeItem('auth_token')
        localStorage.removeItem('auth_user')
        set({ token: null, user: null })
      },

      isAuthenticated: () => Boolean(get().token),

      isAdmin: () => Boolean(get().user?.is_admin),
    }),
    {
      name: 'auth_store',
      partialize: (state) => ({ token: state.token, user: state.user }),
    },
  ),
)
