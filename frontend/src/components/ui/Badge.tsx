type BadgeVariant = 'green' | 'red' | 'yellow' | 'blue' | 'gray' | 'purple'

const variantClasses: Record<BadgeVariant, string> = {
  green: 'bg-green-100 text-green-700',
  red: 'bg-red-100 text-red-700',
  yellow: 'bg-yellow-100 text-yellow-700',
  blue: 'bg-blue-100 text-blue-700',
  gray: 'bg-gray-100 text-gray-600',
  purple: 'bg-purple-100 text-purple-700',
}

interface BadgeProps {
  label: string
  variant?: BadgeVariant
}

export function Badge({ label, variant = 'gray' }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${variantClasses[variant]}`}
    >
      {label}
    </span>
  )
}

export function subscriptionStatusBadge(status: string) {
  const map: Record<string, BadgeVariant> = {
    active: 'green',
    trial: 'blue',
    expired: 'red',
    canceled: 'gray',
  }
  return <Badge label={status} variant={map[status] ?? 'gray'} />
}

export function ticketStatusBadge(status: string) {
  const map: Record<string, BadgeVariant> = {
    open: 'blue',
    answered: 'yellow',
    closed: 'gray',
  }
  return <Badge label={status} variant={map[status] ?? 'gray'} />
}

export function paymentStatusBadge(status: string) {
  const map: Record<string, BadgeVariant> = {
    CONFIRMED: 'green',
    PENDING: 'yellow',
    CANCELED: 'red',
    CHARGEBACKED: 'red',
  }
  return <Badge label={status} variant={map[status] ?? 'gray'} />
}
