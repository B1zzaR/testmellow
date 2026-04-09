import { useProfile } from '@/hooks/useProfile'
import { useAuthStore } from '@/store/authStore'
import { useThemeStore } from '@/store/themeStore'
import { Icon, SnakeLogo } from '@/components/ui/Icons'
import { useInstallPrompt } from '@/hooks/useInstallPrompt'

export function Navbar() {
  const { data: profile } = useProfile()
  const isAdmin = useAuthStore((s) => s.isAdmin())
  const mode = useThemeStore((s) => s.mode)
  const toggleMode = useThemeStore((s) => s.toggleMode)
  const { canInstall, install } = useInstallPrompt()

  return (
    <header className="flex h-14 shrink-0 items-center justify-between border-b border-gray-200 bg-white px-4 dark:border-surface-700 dark:bg-surface-900 lg:h-16 lg:px-5">
      {/* Mobile: logo mark only */}
      <div className="flex items-center gap-2 lg:hidden">
        <SnakeLogo size={28} />
        <span className="text-sm font-bold tracking-wide text-gray-900 dark:text-slate-100">MelloWPN</span>
      </div>

      <div className="hidden lg:block" />

      {/* Right: meta info */}
      <div className="flex flex-wrap items-center justify-end gap-2">
        {/* Install PWA button — Chrome/Android only, hidden when already installed */}
        {canInstall && (
          <button
            onClick={install}
            className="flex items-center gap-1.5 rounded-lg border border-primary-700/50 bg-primary-500/10 px-3 py-1.5 text-xs font-semibold text-primary-400 transition-colors hover:bg-primary-500/20"
            title="Установить приложение на устройство"
          >
            <Icon name="download" size={14} />
            <span className="hidden sm:block">Установить</span>
          </button>
        )}

        {/* Theme toggle */}
        <button
          onClick={toggleMode}
          className="flex h-9 w-9 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 dark:text-slate-500 dark:hover:bg-surface-700"
          aria-label="Переключить тему"
        >
          <Icon name={mode === 'dark' ? 'sun' : 'moon'} size={18} />
        </button>

        {/* Admin badge */}
        {isAdmin && (
          <span className="hidden rounded-md bg-yellow-500/10 px-2.5 py-1 text-sm font-semibold text-yellow-500 sm:block">
            Админ
          </span>
        )}

        {/* YAD balance */}
        {profile && (
          <div className="flex items-center gap-1.5 rounded-lg border border-primary-900/40 bg-primary-500/5 px-3 py-1.5">
            <Icon name="skull" size={16} className="text-primary-500" />
            <span className="text-sm font-semibold text-primary-400">{profile.yad_balance} ЯД</span>
          </div>
        )}

        {/* User email — desktop only */}
        {profile && (
          <span className="hidden max-w-[160px] truncate text-sm text-gray-500 dark:text-slate-500 lg:block">
            {profile.email ?? profile.username ?? 'User'}
          </span>
        )}
      </div>
    </header>
  )
}
