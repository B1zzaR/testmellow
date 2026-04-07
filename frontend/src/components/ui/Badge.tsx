type BadgeVariant = 'green' | 'red' | 'yellow' | 'blue' | 'gray' | 'purple'

const variantClasses: Record<BadgeVariant, string> = {
  green:  'bg-primary-500/15 text-primary-400 dark:ring-1 dark:ring-primary-500/30',
  red:    'bg-red-500/15 text-red-400 dark:ring-1 dark:ring-red-500/30',
  yellow: 'bg-yellow-500/15 text-yellow-400 dark:ring-1 dark:ring-yellow-500/30',
  blue:   'bg-blue-500/15 text-blue-400 dark:ring-1 dark:ring-blue-500/30',
  gray:   'bg-slate-500/15 text-slate-400 dark:ring-1 dark:ring-slate-500/30',
  purple: 'bg-purple-500/15 text-purple-400 dark:ring-1 dark:ring-purple-500/30',
}

interface BadgeProps {
  label: string
  variant?: BadgeVariant
}

export function Badge({ label, variant = 'gray' }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-3 py-1 text-sm font-medium capitalize ${variantClasses[variant]}`}
    >
      {label}
    </span>
  )
}

export function subscriptionStatusBadge(status: string) {
  const map: Record<string, BadgeVariant> = {
    active: 'green', trial: 'blue', expired: 'red', canceled: 'gray',
  }
  const labels: Record<string, string> = {
    active: 'Активна', trial: 'Пробная', expired: 'Истекла', canceled: 'Отменена',
  }
  return <Badge label={labels[status] ?? status} variant={map[status] ?? 'gray'} />
}

export function ticketStatusBadge(status: string) {
  const map: Record<string, BadgeVariant> = {
    open: 'blue', answered: 'yellow', closed: 'gray',
  }
  const labels: Record<string, string> = {
    open: 'Открыт', answered: 'Отвечен', closed: 'Закрыт',
  }
  return <Badge label={labels[status] ?? status} variant={map[status] ?? 'gray'} />
}

export function paymentStatusBadge(status: string) {
  const map: Record<string, BadgeVariant> = {
    CONFIRMED: 'green', PENDING: 'yellow', CANCELED: 'red', CHARGEBACKED: 'red', EXPIRED: 'gray',
  }
  const labels: Record<string, string> = {
    CONFIRMED: 'Оплачено', PENDING: 'Ожидает', CANCELED: 'Отменён', CHARGEBACKED: 'Возврат', EXPIRED: 'Истёк',
  }
  return <Badge label={labels[status] ?? status} variant={map[status] ?? 'gray'} />
}
