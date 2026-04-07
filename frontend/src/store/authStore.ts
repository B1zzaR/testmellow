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

      isAuthenticated: () => {
        const token = get().token
        if (!token) return false
        try {
          const payload = JSON.parse(atob(token.split('.')[1]))
          if (payload.exp && payload.exp * 1000 < Date.now()) {
            set({ token: null, user: null })
            localStorage.removeItem('auth_token')
            return false
          }
          return true
        } catch {
          return false
        }
      },

      isAdmin: () => Boolean(get().user?.is_admin),
    }),
    {
      name: 'auth_store',
      partialize: (state) => ({ token: state.token, user: state.user }),
    },
  ),
)
