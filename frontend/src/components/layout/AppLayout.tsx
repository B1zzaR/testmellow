import { Outlet } from 'react-router-dom'
import { BottomNav, Sidebar } from './Sidebar'
import { Navbar } from './Navbar'
import { useOnlineStatus } from '@/hooks/useOnlineStatus'

export function AppLayout() {
  const isOnline = useOnlineStatus()

  return (
    <div className="flex h-screen h-[100dvh] overflow-hidden bg-gradient-to-b from-gray-50 to-white dark:from-surface-950 dark:to-surface-950 dark:bg-surface-950">
      {/* Sidebar — hidden on mobile, static on ≥lg */}
      <div className="hidden lg:flex lg:w-64 lg:shrink-0">
        <Sidebar />
      </div>

      {/* Main */}
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <Navbar />

        {!isOnline && (
          <div className="flex items-center justify-center gap-2 bg-yellow-500/10 px-4 py-2 text-sm font-medium text-yellow-700 dark:text-yellow-300">
            <span>⚠</span>
            <span>Нет подключения к интернету — данные могут быть устаревшими</span>
          </div>
        )}

        {/* pb on mobile clears the bottom nav (~56px) + safe-area-inset; lg drops the extra space */}
        <main className="flex-1 overflow-y-auto p-4 pb-[calc(env(safe-area-inset-bottom,0px)+6.5rem)] text-gray-900 dark:text-slate-100 scrollbar-thin mobile-scroll md:p-6 lg:pb-6">
          <Outlet />
        </main>
      </div>

      {/* Mobile bottom nav */}
      <BottomNav />
    </div>
  )
}
