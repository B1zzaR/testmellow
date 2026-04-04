import { useProfile } from '@/hooks/useProfile'
import { useLogout } from '@/hooks/useAuth'
import { useAuthStore } from '@/store/authStore'
import { useThemeStore } from '@/store/themeStore'
import { Link } from 'react-router-dom'

export function Navbar() {
  const { data: profile } = useProfile()
  const logout = useLogout()
  const isAdmin = useAuthStore((s) => s.isAdmin())
  const mode = useThemeStore((s) => s.mode)
  const toggleMode = useThemeStore((s) => s.toggleMode)

  return (
    <header className="flex h-16 items-center justify-between border-b border-gray-200 bg-white px-6 dark:border-slate-700 dark:bg-slate-900">
      <div />
      <div className="flex items-center gap-4">
        <button
          onClick={toggleMode}
          className="rounded-md border border-gray-200 bg-gray-50 px-3 py-1 text-xs font-semibold text-gray-700 hover:bg-gray-100 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
        >
          {mode === 'dark' ? 'Light' : 'Dark'}
        </button>
        {isAdmin && (
          <Link
            to="/admin"
            className="rounded-md bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-700 hover:bg-amber-200 dark:bg-amber-900/40 dark:text-amber-300 dark:hover:bg-amber-900/60"
          >
            Admin Panel
          </Link>
        )}
        {profile && (
          <span className="text-sm text-gray-600 dark:text-slate-300">
            {profile.email ?? profile.username ?? 'User'}
          </span>
        )}
        {profile && (
          <span className="rounded-full bg-primary-50 px-3 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">
            {profile.yad_balance} YAD
          </span>
        )}
        <button
          onClick={logout}
          className="rounded-md px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-slate-100"
        >
          Logout
        </button>
      </div>
    </header>
  )
}
