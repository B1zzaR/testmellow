import { Navigate } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'

interface PublicRouteProps {
  children: React.ReactNode
}

export function PublicRoute({ children }: PublicRouteProps) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated())

  // Redirect authenticated users to dashboard
  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />
  }

  // Show public page for non-authenticated users
  return <>{children}</>
}
