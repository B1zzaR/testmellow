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
  const token = useAuthStore((s) => s.token)
  const setAuth = useAuthStore((s) => s.setAuth)
  const mode = useThemeStore((s) => s.mode)

  useEffect(() => {
    document.documentElement.classList.toggle('dark', mode === 'dark')
  }, [mode])

  useEffect(() => {
    if (!token) return

    let canceled = false
    profileApi
      .getProfile()
      .then((profile) => {
        if (canceled) return
        setAuth(token, {
          id: profile.id,
          is_admin: profile.is_admin,
          email: profile.email,
        })
      })
      .catch(() => {
        // Ignore bootstrap errors: route guards and interceptors handle invalid sessions.
      })

    return () => {
      canceled = true
    }
  }, [token, setAuth])

  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}
