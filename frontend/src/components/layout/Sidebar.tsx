import { useState } from 'react'
import { NavLink } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'
import { useLogout } from '@/hooks/useAuth'
import { Icon, SnakeLogo } from '@/components/ui/Icons'

type IconName = Parameters<typeof Icon>[0]['name']

interface NavItem {
  to: string
  label: string
  icon: IconName
  end?: boolean
}

const navItems: NavItem[] = [
  { to: '/dashboard',        label: 'Обзор',           icon: 'dashboard',  end: true },
  { to: '/subscriptions',    label: 'Подписки',        icon: 'shield'               },
  { to: '/referrals',        label: 'Рефералы',        icon: 'users'                },
  { to: '/shop',             label: 'Магазин ЯД',      icon: 'shop'                 },
  { to: '/tickets',          label: 'Поддержка',       icon: 'message'              },
  { to: '/promo',            label: 'Промокод',        icon: 'tag'                  },
  { to: '/payments/history', label: 'Платежи',         icon: 'coins'                },
  { to: '/settings',         label: 'Настройки',       icon: 'sliders'              },
]

// Bottom nav shows first 4 items; remaining items go into the "Ещё" sheet
const bottomNavItems = navItems.slice(0, 4)
const moreNavItems = navItems.slice(4) // [Поддержка, Промокод, Платежи, Настройки]

interface SidebarProps {
  onClose?: () => void
}

export function Sidebar({ onClose }: SidebarProps) {
  const isAdmin = useAuthStore((s) => s.isAdmin())
  const logout = useLogout()

  const handleLogout = () => {
    logout()
    onClose?.()
  }

  return (
    <aside className="flex h-full w-64 flex-col bg-white shadow-elevation-1 dark:bg-surface-900 border-r border-gray-200 dark:border-surface-700 dark:shadow-none">
      {/* Brand */}
      <div className="flex h-[72px] items-center gap-3 border-b border-gray-200 dark:border-surface-700 px-5">
        <SnakeLogo size={36} />
        <p className="text-base font-bold tracking-wide text-gray-900 dark:text-slate-100">MelloWPN</p>
      </div>

      {/* Nav */}
      <nav className="flex-1 overflow-y-auto px-3 py-4 scrollbar-thin space-y-0.5">
        <p className="mb-2 px-3 text-xs font-semibold uppercase tracking-widest text-gray-400 dark:text-slate-600">
          Навигация
        </p>
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            onClick={onClose}
            className={({ isActive }) =>
              [
                'group flex items-center gap-3 rounded-lg px-3 py-3 text-sm font-medium transition-all',
                isActive
                  ? 'bg-primary-500/10 text-primary-500 dark:text-primary-400'
                  : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-slate-400 dark:hover:bg-surface-700 dark:hover:text-slate-100',
              ].join(' ')
            }
          >
            {({ isActive }) => (
              <>
                <Icon
                  name={item.icon}
                  size={18}
                  className={isActive ? 'text-primary-500' : 'text-gray-400 dark:text-slate-600 group-hover:text-gray-600 dark:group-hover:text-slate-300'}
                />
                {item.label}
                {isActive && (
                  <span className="ml-auto h-2 w-2 rounded-full bg-primary-500 shadow-glow-sm" />
                )}
              </>
            )}
          </NavLink>
        ))}

        {/* Admin section */}
        {isAdmin && (
          <>
            <p className="mb-2 mt-5 px-3 text-xs font-semibold uppercase tracking-widest text-gray-400 dark:text-slate-600">
              Администрирование
            </p>
            <NavLink
              to="/admin"
              onClick={onClose}
              className={({ isActive }) =>
                [
                  'group flex items-center gap-3 rounded-lg px-3 py-3 text-sm font-medium transition-all',
                  isActive
                    ? 'bg-yellow-500/10 text-yellow-500'
                    : 'text-gray-600 hover:bg-gray-100 dark:text-slate-400 dark:hover:bg-surface-700 dark:hover:text-slate-100',
                ].join(' ')
              }
            >
              <Icon name="settings" size={18} className="text-yellow-500/60 group-hover:text-yellow-500 transition-colors" />
              Панель администратора
            </NavLink>
          </>
        )}
      </nav>

      {/* Logout */}
      <div className="border-t border-gray-200 dark:border-surface-700 px-3 py-3">
        <button
          onClick={handleLogout}
          className="flex w-full items-center gap-3 rounded-lg px-3 py-3 text-sm font-medium text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:text-slate-500 dark:hover:bg-red-500/10 dark:hover:text-red-400"
        >
          <Icon name="logout" size={18} />
          Выйти
        </button>
      </div>
    </aside>
  )
}

/** Mobile bottom navigation bar — rendered separately in AppLayout */
export function BottomNav() {
  const [showMore, setShowMore] = useState(false)
  const isAdmin = useAuthStore((s) => s.isAdmin())
  const logout = useLogout()

  return (
    <>
      {/* "Ещё" bottom sheet */}
      {showMore && (
        <div
          className="fixed inset-0 z-40 flex flex-col justify-end lg:hidden"
          onClick={() => setShowMore(false)}
        >
          {/* Backdrop */}
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" />

          {/* Sheet panel */}
          <div
            className="relative z-50 rounded-t-2xl border-t border-gray-200 bg-white dark:border-surface-700 dark:bg-surface-900 px-4 pt-4 pb-6 animate-modal-up"
            onClick={(e) => e.stopPropagation()}
            style={{ paddingBottom: 'calc(env(safe-area-inset-bottom, 0px) + 1.5rem)' }}
          >
            {/* Drag handle */}
            <div className="mx-auto mb-4 h-1 w-10 rounded-full bg-gray-300 dark:bg-surface-600" />

            <p className="mb-2 px-3 text-xs font-semibold uppercase tracking-widest text-gray-400 dark:text-slate-600">
              Навигация
            </p>

            <div className="space-y-0.5">
              {moreNavItems.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  onClick={() => setShowMore(false)}
                  className={({ isActive }) =>
                    [
                      'flex items-center gap-3 rounded-lg px-3 py-3 text-sm font-medium transition-all',
                      isActive
                        ? 'bg-primary-500/10 text-primary-500 dark:text-primary-400'
                        : 'text-gray-600 dark:text-slate-400',
                    ].join(' ')
                  }
                >
                  {({ isActive }) => (
                    <>
                      <Icon
                        name={item.icon}
                        size={18}
                        className={isActive ? 'text-primary-500' : 'text-gray-400 dark:text-slate-600'}
                      />
                      {item.label}
                      {isActive && <span className="ml-auto h-2 w-2 rounded-full bg-primary-500 shadow-glow-sm" />}
                    </>
                  )}
                </NavLink>
              ))}

              {isAdmin && (
                <NavLink
                  to="/admin"
                  onClick={() => setShowMore(false)}
                  className={({ isActive }) =>
                    [
                      'flex items-center gap-3 rounded-lg px-3 py-3 text-sm font-medium transition-all',
                      isActive
                        ? 'bg-yellow-500/10 text-yellow-500'
                        : 'text-gray-600 dark:text-slate-400',
                    ].join(' ')
                  }
                >
                  <Icon name="settings" size={18} className="text-yellow-500/60" />
                  Панель администратора
                </NavLink>
              )}
            </div>

            <div className="mt-4 border-t border-gray-100 dark:border-surface-700 pt-3">
              <button
                onClick={() => { logout(); setShowMore(false) }}
                className="flex w-full items-center gap-3 rounded-lg px-3 py-3 text-sm font-medium text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:text-slate-500 dark:hover:bg-red-500/10 dark:hover:text-red-400"
              >
                <Icon name="logout" size={18} />
                Выйти
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Bottom tab bar */}
      <nav
        className={[
          'fixed bottom-0 inset-x-0 z-30 lg:hidden',
          'flex items-stretch border-t border-gray-200 bg-white',
          'dark:border-surface-700 dark:bg-surface-900',
          'pb-[env(safe-area-inset-bottom)]',
        ].join(' ')}
      >
        {bottomNavItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            className={({ isActive }) =>
              [
                'flex flex-1 flex-col items-center justify-center gap-0.5 py-2',
                'min-h-[56px] select-none text-[10px] font-medium transition-colors',
                isActive
                  ? 'text-primary-500'
                  : 'text-gray-400 dark:text-slate-600',
              ].join(' ')
            }
          >
            {({ isActive }) => (
              <>
                <Icon
                  name={item.icon}
                  size={20}
                  className={isActive ? 'text-primary-500' : 'text-gray-400 dark:text-slate-500'}
                />
                <span className="whitespace-nowrap">{item.label}</span>
              </>
            )}
          </NavLink>
        ))}

        {/* "Ещё" button — opens sheet with remaining nav items */}
        <button
          onClick={() => setShowMore((v) => !v)}
          aria-label="Ещё"
          aria-expanded={showMore}
          className={[
            'flex flex-1 flex-col items-center justify-center gap-0.5 py-2',
            'min-h-[56px] select-none text-[10px] font-medium transition-colors',
            showMore ? 'text-primary-500' : 'text-gray-400 dark:text-slate-600',
          ].join(' ')}
        >
          <Icon
            name="menu"
            size={20}
            className={showMore ? 'text-primary-500' : 'text-gray-400 dark:text-slate-500'}
          />
          <span className="whitespace-nowrap">Ещё</span>
        </button>
      </nav>
    </>
  )
}
