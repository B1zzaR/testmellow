import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router-dom'
import { router } from '@/router'
import { useEffect } from 'react'
import { profileApi } from '@/api/profile'
import { useAuthStore } from '@/store/authStore'
import { useThemeStore } from '@/store/themeStore'
import { Spinner } from '@/components/ui/Spinner'

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

function RootInitializer() {
  const setAuth = useAuthStore((s) => s.setAuth)
  const clearAuth = useAuthStore((s) => s.clearAuth)
  const setInitialized = useAuthStore((s) => s.setInitialized)
  const initialized = useAuthStore((s) => s.initialized)
  const mode = useThemeStore((s) => s.mode)

  useEffect(() => {
    document.documentElement.classList.toggle('dark', mode === 'dark')
  }, [mode])

  // On mount, verify the session via the HttpOnly cookie.
  // If the cookie is valid the server returns the user profile; otherwise 401 clears state.
  useEffect(() => {
    let canceled = false

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
  }, [setAuth, clearAuth, setInitialized])

  // Don't render anything until auth is initialized
  if (!initialized) {
    return (
      <div className="flex items-center justify-center h-screen bg-surface-950">
        <Spinner />
      </div>
    )
  }

  return <RouterProvider router={router} />
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RootInitializer />
    </QueryClientProvider>
  )
}
