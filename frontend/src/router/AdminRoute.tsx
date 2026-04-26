import { Outlet } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import { NotFoundPage } from '@/pages/NotFoundPage'

// Show 404 for both unauthenticated visitors AND authenticated non-admins.
// Non-admins get the same opaque "page not found" so the existence of the
// admin panel is not advertised — defence in depth on top of the AdminDBCheck
// middleware on the API.
export function AdminRoute() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated())
  const isAdmin = useAuthStore((s) => s.isAdmin())
  if (!isAuthenticated || !isAdmin) {
    return <NotFoundPage />
  }
  return <Outlet />
}
