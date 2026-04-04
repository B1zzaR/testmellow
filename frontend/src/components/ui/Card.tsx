import type { ReactNode } from 'react'

interface CardProps {
  children: ReactNode
  className?: string
  title?: string
  subtitle?: string
  action?: ReactNode
}

export function Card({ children, className = '', title, subtitle, action }: CardProps) {
  return (
    <div className={`rounded-xl border border-gray-200 bg-white shadow-sm dark:border-slate-700 dark:bg-slate-900 ${className}`}>
      {(title || action) && (
        <div className="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-slate-700">
          <div>
            {title && <h3 className="text-base font-semibold text-gray-900 dark:text-slate-100">{title}</h3>}
            {subtitle && <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-400">{subtitle}</p>}
          </div>
          {action && <div>{action}</div>}
        </div>
      )}
      <div className="px-6 py-4">{children}</div>
    </div>
  )
}

interface StatCardProps {
  label: string
  value: string | number
  sub?: string
  icon?: string
}

export function StatCard({ label, value, sub, icon }: StatCardProps) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-slate-700 dark:bg-slate-900">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm text-gray-500 dark:text-slate-400">{label}</p>
          <p className="mt-1 text-2xl font-semibold text-gray-900 dark:text-slate-100">{value}</p>
          {sub && <p className="mt-1 text-xs text-gray-400 dark:text-slate-500">{sub}</p>}
        </div>
        {icon && <span className="text-2xl">{icon}</span>}
      </div>
    </div>
  )
}
