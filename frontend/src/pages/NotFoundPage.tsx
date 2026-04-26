import { Link } from 'react-router-dom'
import { Button } from '@/components/ui/Button'
import { Icon } from '@/components/ui/Icons'
import { useAuthStore } from '@/store/authStore'

export function NotFoundPage() {
  // Show different primary actions depending on auth state. PrivateRoute and
  // AdminRoute render this same page when the visitor is unauthenticated, so
  // the action that gets surfaced first should be "Sign in", not "Go to
  // dashboard" (which would just bounce back here).
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated())

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 dark:bg-surface-900 px-4 text-center">
      <div className="mb-6 text-slate-300 dark:text-slate-700">
        <Icon name="shield" size={64} />
      </div>
      <h1 className="text-6xl font-extrabold text-gray-900 dark:text-slate-100">404</h1>
      <p className="mt-3 text-lg text-gray-500 dark:text-slate-400">Страница не найдена</p>
      <p className="mt-1 text-sm text-gray-400 dark:text-slate-600">
        Такой страницы не существует или она была перемещена.
      </p>
      <div className="mt-8 flex gap-3">
        {isAuthenticated ? (
          <Link to="/dashboard">
            <Button>На главную</Button>
          </Link>
        ) : (
          <Link to="/login">
            <Button>Войти</Button>
          </Link>
        )}
        <Link to="/">
          <Button variant="secondary">На лэндинг</Button>
        </Link>
      </div>
    </div>
  )
}
