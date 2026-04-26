import { Outlet } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import { NotFoundPage } from '@/pages/NotFoundPage'

// Render the 404 page for unauthenticated users instead of redirecting to
// /login. The previous Navigate-to-login flow combined with stale
// localStorage and the api-interceptor's hard reload created an infinite
// reload loop on mobile when a user opened a protected URL with expired
// cookies. Returning a static page short-circuits the loop: NotFoundPage
// has no auth-dependent fetches, so the cycle terminates.
export function PrivateRoute() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated())
  if (!isAuthenticated) {
    return <NotFoundPage />
  }
  return <Outlet />
}
