import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface AuthUser {
  id: string
  is_admin: boolean
  email: string | null
}

interface AuthState {
  user: AuthUser | null
  setAuth: (user: AuthUser) => void
  clearAuth: () => void
  isAuthenticated: () => boolean
  isAdmin: () => boolean
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,

      // Tokens are in HttpOnly cookies — only persist user metadata.
      setAuth: (user) => set({ user }),

      clearAuth: () => set({ user: null }),

      isAuthenticated: () => get().user !== null,

      isAdmin: () => Boolean(get().user?.is_admin),
    }),
    {
      name: 'auth_store',
      partialize: (state) => ({ user: state.user }),
    },
  ),
)
