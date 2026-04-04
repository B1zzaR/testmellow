import { NavLink } from 'react-router-dom'

interface NavItem {
  to: string
  label: string
  icon: string
}

const adminNavItems: NavItem[] = [
  { to: '/admin', label: 'Dashboard', icon: '📊' },
  { to: '/admin/users', label: 'Users', icon: '👤' },
  { to: '/admin/tickets', label: 'Tickets', icon: '🎫' },
  { to: '/admin/promo', label: 'Promo Codes', icon: '🏷' },
]

export function AdminSidebar() {
  return (
    <aside className="flex w-60 flex-col border-r border-gray-200 bg-white dark:border-slate-700 dark:bg-slate-900">
      <div className="flex h-16 items-center border-b border-gray-200 px-6 dark:border-slate-700">
        <span className="text-xl font-bold text-primary-600">Admin Panel</span>
      </div>
      <nav className="flex-1 space-y-1 px-3 py-4">
        {adminNavItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.to === '/admin'}
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
      <div className="border-t border-gray-200 px-3 py-3 dark:border-slate-700">
        <NavLink
          to="/dashboard"
          className="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-gray-600 hover:bg-gray-100 dark:text-slate-300 dark:hover:bg-slate-800"
        >
          <span>↩</span> User Area
        </NavLink>
      </div>
    </aside>
  )
}
