import { NavLink } from 'react-router-dom'
import { Icon, SnakeLogo } from '@/components/ui/Icons'

type IconName = Parameters<typeof Icon>[0]['name']

interface NavItem {
  to: string
  label: string
  icon: IconName
  end?: boolean
}

const adminNavItems: NavItem[] = [
  { to: '/admin',               label: 'Дашборд',          icon: 'chart',     end: true },
  { to: '/admin/users',         label: 'Пользователи',     icon: 'users'                },
  { to: '/admin/payments',      label: 'Платежи',          icon: 'coins'                },
  { to: '/admin/subscriptions', label: 'Подписки',         icon: 'shield'               },
  { to: '/admin/referrals',     label: 'Рефералы',         icon: 'gem'                  },
  { to: '/admin/yad',           label: 'ЯД-экономика',    icon: 'diamond'              },
  { to: '/admin/tickets',       label: 'Тикеты',           icon: 'ticket'               },
  { to: '/admin/promo',         label: 'Промокоды',        icon: 'tag'                  },
  { to: '/admin/notifications', label: 'Уведомления',      icon: 'bell'                 },
  { to: '/admin/broadcast',     label: 'Рассылка',         icon: 'megaphone'            },
  { to: '/admin/suggestions',   label: 'Предложения',      icon: 'lightbulb'            },
]

interface AdminSidebarProps {
  onClose?: () => void
}

export function AdminSidebar({ onClose }: AdminSidebarProps) {
  return (
    <aside className="flex h-full w-64 flex-col bg-white dark:bg-surface-900 border-r border-gray-200 dark:border-surface-700">
      {/* Brand */}
      <div className="flex h-16 items-center gap-3 border-b border-gray-200 dark:border-surface-700 px-5">
        <SnakeLogo size={30} />
        <p className="text-sm font-bold tracking-wide text-gray-900 dark:text-slate-100">Панель администратора</p>
      </div>

      {/* Nav */}
      <nav className="flex-1 overflow-y-auto px-3 py-4 scrollbar-thin space-y-0.5">
        <p className="mb-2 px-3 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-slate-600">
          Управление
        </p>
        {adminNavItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            onClick={onClose}
            className={({ isActive }) =>
              [
                'group flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-all',
                isActive
                  ? 'bg-yellow-500/10 text-yellow-500'
                  : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-slate-400 dark:hover:bg-surface-700 dark:hover:text-slate-100',
              ].join(' ')
            }
          >
            {({ isActive }) => (
              <>
                <Icon
                  name={item.icon}
                  size={16}
                  className={isActive ? 'text-yellow-500' : 'text-gray-400 dark:text-slate-600 group-hover:text-gray-600 dark:group-hover:text-slate-300'}
                />
                {item.label}
              </>
            )}
          </NavLink>
        ))}
      </nav>

      {/* Back to user area */}
      <div className="border-t border-gray-200 dark:border-surface-700 px-3 py-3">
        <NavLink
          to="/dashboard"
          onClick={onClose}
          className="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-gray-500 transition-colors hover:bg-gray-100 dark:text-slate-500 dark:hover:bg-surface-700 dark:hover:text-slate-200"
        >
          <Icon name="back" size={16} />
          Личный кабинет
        </NavLink>
      </div>
    </aside>
  )
}
