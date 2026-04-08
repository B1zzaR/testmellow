import { Navigate, Outlet } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import { Spinner } from '@/components/ui/Spinner'

export function PrivateRoute() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated())
  const initialized = useAuthStore((s) => s.initialized)

  if (!initialized) {
    return (
      <div className="flex items-center justify-center h-screen">
        <Spinner />
      </div>
    )
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  return <Outlet />
}
