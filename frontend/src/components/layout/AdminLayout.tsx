import { useState } from 'react'
import { Outlet } from 'react-router-dom'
import { AdminSidebar } from './AdminSidebar'
import { Navbar } from './Navbar'
import { Icon } from '@/components/ui/Icons'

export function AdminLayout() {
  const [sidebarOpen, setSidebarOpen] = useState(false)

  return (
    <div className="flex h-screen overflow-hidden bg-gradient-to-b from-gray-50 to-white dark:from-surface-950 dark:to-surface-950 dark:bg-surface-950">
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-30 bg-black/60 backdrop-blur-sm lg:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}
      <div
        className={[
          'fixed inset-y-0 left-0 z-40 w-64 transition-transform duration-300 ease-in-out',
          'lg:relative lg:z-auto lg:translate-x-0',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full',
        ].join(' ')}
      >
        <AdminSidebar onClose={() => setSidebarOpen(false)} />
      </div>
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        {/* Admin-specific header with hamburger on mobile */}
        <div className="relative flex shrink-0">
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="absolute left-4 top-1/2 z-10 flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 dark:text-slate-500 dark:hover:bg-surface-700 lg:hidden"
            aria-label="Открыть меню"
          >
            <Icon name="menu" size={20} />
          </button>
          <div className="flex-1 pl-12 lg:pl-0">
            <Navbar />
          </div>
        </div>
        <main className="flex-1 overflow-y-auto p-4 text-gray-900 dark:text-slate-100 scrollbar-thin md:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
