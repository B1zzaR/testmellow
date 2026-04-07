import type { ReactNode } from 'react'

interface CardProps {
  children: ReactNode
  className?: string
  title?: string
  subtitle?: string
  action?: ReactNode
  glow?: boolean
}

export function Card({ children, className = '', title, subtitle, action, glow }: CardProps) {
  return (
    <div
      className={[
        'rounded-xl border bg-white shadow-sm',
        'dark:bg-surface-900 dark:border-surface-700 dark:shadow-card',
        glow ? 'dark:shadow-glow-sm dark:border-primary-900/60' : '',
        className,
      ].join(' ')}
    >
      {(title || action) && (
        <div className="flex items-center justify-between border-b border-gray-100 px-4 py-3.5 sm:px-6 sm:py-4 dark:border-surface-700">
          <div>
            {title && (
              <h3 className="text-base font-semibold tracking-wide text-gray-900 dark:text-slate-100">
                {title}
              </h3>
            )}
            {subtitle && (
              <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">{subtitle}</p>
            )}
          </div>
          {action && <div>{action}</div>}
        </div>
      )}
      <div className="px-4 py-4 sm:px-6 sm:py-5">{children}</div>
    </div>
  )
}

interface StatCardProps {
  label: string
  value: string | number
  sub?: string
  icon?: ReactNode
  accent?: boolean
}

export function StatCard({ label, value, sub, icon, accent }: StatCardProps) {
  return (
    <div
      className={[
        'rounded-xl border p-4 sm:p-6',
        'bg-white dark:bg-surface-900 dark:border-surface-700 dark:shadow-card',
        accent ? 'dark:border-primary-900/60 dark:shadow-glow-sm' : '',
      ].join(' ')}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-slate-500">
            {label}
          </p>
          <p className="mt-2 text-3xl font-bold text-gray-900 dark:text-slate-100 sm:text-4xl">{value}</p>
          {sub && <p className="mt-1.5 text-sm text-gray-400 dark:text-slate-600">{sub}</p>}
        </div>
        {icon && (
          <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary-500/10 text-primary-500 sm:h-14 sm:w-14 [&>svg]:!h-5 [&>svg]:!w-5 sm:[&>svg]:!h-7 sm:[&>svg]:!w-7">
            {icon}
          </div>
        )}
      </div>
    </div>
  )
}
