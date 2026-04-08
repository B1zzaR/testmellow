import { Navigate } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import { Spinner } from '@/components/ui/Spinner'
import { profileApi } from '@/api/profile'
import { useEffect, useState } from 'react'

interface PublicRouteProps {
  children: React.ReactNode
}

export function PublicRoute({ children }: PublicRouteProps) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated())
  const setAuth = useAuthStore((s) => s.setAuth)
  const clearAuth = useAuthStore((s) => s.clearAuth)
  const [initialized, setInitialized] = useState(false)

  useEffect(() => {
    // If already has user in store and not yet checked, verify it's still valid
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
  }, [setAuth, clearAuth])

  if (!initialized) {
    return (
      <div className="flex items-center justify-center h-screen bg-surface-950">
        <Spinner />
      </div>
    )
  }

  // Redirect authenticated users to dashboard
  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />
  }

  return <>{children}</>
}
