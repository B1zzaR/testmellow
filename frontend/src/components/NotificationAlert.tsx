import { Icon } from '@/components/ui/Icons'
import type { SystemNotification } from '@/api/types'

interface NotificationAlertProps {
  notification: SystemNotification
  onDismiss?: () => void
}

const typeStyles = {
  warning: {
    bg: 'bg-yellow-50',
    border: 'border-yellow-200',
    title: 'text-yellow-900',
    message: 'text-yellow-800',
    icon: 'text-yellow-600',
    iconName: 'tag' as const,
  },
  error: {
    bg: 'bg-red-50',
    border: 'border-red-200',
    title: 'text-red-900',
    message: 'text-red-800',
    icon: 'text-red-600',
    iconName: 'x-circle' as const,
  },
  info: {
    bg: 'bg-blue-50',
    border: 'border-blue-200',
    title: 'text-blue-900',
    message: 'text-blue-800',
    icon: 'text-blue-600',
    iconName: 'message' as const,
  },
  success: {
    bg: 'bg-green-50',
    border: 'border-green-200',
    title: 'text-green-900',
    message: 'text-green-800',
    icon: 'text-green-600',
    iconName: 'check-circle' as const,
  },
}

export function NotificationAlert({ notification, onDismiss }: NotificationAlertProps) {
  const style = typeStyles[notification.type]

  return (
    <div className={`rounded-lg border ${style.bg} ${style.border} p-4 mb-4`}>
      <div className="flex items-start gap-3">
        <Icon name={style.iconName} size={20} className={`mt-0.5 flex-shrink-0 ${style.icon}`} />
        <div className="flex-1 min-w-0">
          <h3 className={`font-semibold text-sm ${style.title}`}>{notification.title}</h3>
          <p className={`text-sm mt-1 ${style.message}`}>{notification.message}</p>
        </div>
        {onDismiss && (
          <button
            onClick={onDismiss}
            className="flex-shrink-0 text-gray-400 hover:text-gray-600 ml-2 mt-1"
            aria-label="Dismiss notification"
          >
            <Icon name="close" size={16} />
          </button>
        )}
      </div>
    </div>
  )
}
