type AlertVariant = 'success' | 'error' | 'warning' | 'info'

const styles: Record<AlertVariant, string> = {
  success: 'bg-green-50 border-green-200 text-green-800 dark:bg-green-900/20 dark:border-green-900 dark:text-green-300',
  error: 'bg-red-50 border-red-200 text-red-800 dark:bg-red-900/20 dark:border-red-900 dark:text-red-300',
  warning: 'bg-yellow-50 border-yellow-200 text-yellow-800 dark:bg-yellow-900/20 dark:border-yellow-900 dark:text-yellow-300',
  info: 'bg-blue-50 border-blue-200 text-blue-800 dark:bg-blue-900/20 dark:border-blue-900 dark:text-blue-300',
}

const icons: Record<AlertVariant, string> = {
  success: '✓',
  error: '✕',
  warning: '⚠',
  info: 'ℹ',
}

interface AlertProps {
  variant?: AlertVariant
  message: string
  className?: string
}

export function Alert({ variant = 'info', message, className = '' }: AlertProps) {
  return (
    <div
      className={`flex items-start gap-2 rounded-lg border px-4 py-3 text-sm ${styles[variant]} ${className}`}
      role="alert"
    >
      <span className="font-bold">{icons[variant]}</span>
      <span>{message}</span>
    </div>
  )
}
