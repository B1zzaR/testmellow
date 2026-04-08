import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router-dom'
import { router } from '@/router'
import { useEffect } from 'react'
import { profileApi } from '@/api/profile'
import { useAuthStore } from '@/store/authStore'
import { useThemeStore } from '@/store/themeStore'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
    mutations: {
      retry: 0,
    },
  },
})

export default function App() {
  const setAuth = useAuthStore((s) => s.setAuth)
  const clearAuth = useAuthStore((s) => s.clearAuth)
  const mode = useThemeStore((s) => s.mode)

  useEffect(() => {
    document.documentElement.classList.toggle('dark', mode === 'dark')
  }, [mode])

  // On mount, verify the session via the HttpOnly cookie.
  // If the cookie is valid the server returns the user profile; otherwise 401 clears state.
  useEffect(() => {
    let canceled = false
    const setInitialized = useAuthStore.getState().setInitialized
    
    profileApi
      .getProfile()
      .then((profile) => {
        if (canceled) return
        setAuth({
          id: profile.id,
          is_admin: profile.is_admin,
          email: profile.email ?? null,
        })
        setInitialized(true)
      })
      .catch(() => {
        if (canceled) return
        clearAuth()
        setInitialized(true)
      })

    return () => {
      canceled = true
    }
  }, [setAuth, clearAuth]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}
