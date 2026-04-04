import { NavLink } from 'react-router-dom'
import { useAuthStore } from '@/store/authStore'

interface NavItem {
  to: string
  label: string
  icon: string
}

const navItems: NavItem[] = [
  { to: '/dashboard', label: 'Dashboard', icon: '⊞' },
  { to: '/subscriptions', label: 'Subscriptions', icon: '🛡' },
  { to: '/referrals', label: 'Referrals', icon: '👥' },
  { to: '/balance', label: 'YAD Balance', icon: '💎' },
  { to: '/shop', label: 'Shop', icon: '🛒' },
  { to: '/tickets', label: 'Support', icon: '💬' },
  { to: '/promo', label: 'Promo Code', icon: '🎁' },
]

export function Sidebar() {
  const isAdmin = useAuthStore((s) => s.isAdmin())

  return (
    <aside className="flex w-60 flex-col border-r border-gray-200 bg-white dark:border-slate-700 dark:bg-slate-900">
      <div className="flex h-16 items-center border-b border-gray-200 px-6 dark:border-slate-700">
        <span className="text-xl font-bold text-primary-600">VPN Platform</span>
      </div>
      <nav className="flex-1 space-y-1 px-3 py-4">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                isActive
                  ? 'bg-primary-50 text-primary-700'
                  : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-slate-100'
              }`
            }
          >
            <span className="text-base">{item.icon}</span>
            {item.label}
          </NavLink>
        ))}
      </nav>
      {isAdmin && (
        <div className="border-t border-amber-100 px-3 py-3 dark:border-amber-900/40">
          <NavLink
            to="/admin"
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                isActive
                  ? 'bg-amber-50 text-amber-700'
                  : 'text-amber-600 hover:bg-amber-50 hover:text-amber-800'
              }`
            }
          >
            <span className="text-base">⚙️</span>
            Admin Panel
          </NavLink>
        </div>
      )}
    </aside>
  )
}
