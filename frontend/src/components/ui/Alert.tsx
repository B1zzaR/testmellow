type AlertVariant = 'success' | 'error' | 'warning' | 'info'

const styles: Record<AlertVariant, string> = {
  success: 'bg-primary-500/10 border-primary-500/30 text-primary-400',
  error:   'bg-red-500/10 border-red-500/30 text-red-400',
  warning: 'bg-yellow-500/10 border-yellow-500/30 text-yellow-400',
  info:    'bg-blue-500/10 border-blue-500/30 text-blue-400',
}

const iconPaths: Record<AlertVariant, string> = {
  success: 'M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
  error:   'M9.75 9.75l4.5 4.5m0-4.5l-4.5 4.5M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
  warning: 'M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z',
  info:    'M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z',
}

const iconColors: Record<AlertVariant, string> = {
  success: 'text-primary-500',
  error:   'text-red-500',
  warning: 'text-yellow-500',
  info:    'text-blue-400',
}

interface AlertProps {
  variant?: AlertVariant
  message: string
  className?: string
}

export function Alert({ variant = 'info', message, className = '' }: AlertProps) {
  return (
    <div
      className={`flex items-start gap-3 rounded-xl border px-4 py-3 text-sm ${styles[variant]} ${className}`}
      role="alert"
    >
      <svg
        className={`h-4 w-4 shrink-0 mt-0.5 ${iconColors[variant]}`}
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={1.75}
        aria-hidden="true"
      >
        <path strokeLinecap="round" strokeLinejoin="round" d={iconPaths[variant]} />
      </svg>
      <span className="leading-relaxed">{message}</span>
    </div>
  )
}
