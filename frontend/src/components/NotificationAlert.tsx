import { AlertTriangle, AlertCircle, Info, CheckCircle, X } from 'lucide-react'
import type { SystemNotification } from '../api/types'

interface NotificationAlertProps {
  notification: SystemNotification
  onDismiss?: () => void
}

export function NotificationAlert({ notification, onDismiss }: NotificationAlertProps) {
  const typeStyles = {
    warning: {
      bg: 'bg-yellow-50',
      border: 'border-yellow-200',
      title: 'text-yellow-900',
      message: 'text-yellow-800',
      icon: 'text-yellow-600',
      Icon: AlertTriangle,
    },
    error: {
      bg: 'bg-red-50',
      border: 'border-red-200',
      title: 'text-red-900',
      message: 'text-red-800',
      icon: 'text-red-600',
      Icon: AlertCircle,
    },
    info: {
      bg: 'bg-blue-50',
      border: 'border-blue-200',
      title: 'text-blue-900',
      message: 'text-blue-800',
      icon: 'text-blue-600',
      Icon: Info,
    },
    success: {
      bg: 'bg-green-50',
      border: 'border-green-200',
      title: 'text-green-900',
      message: 'text-green-800',
      icon: 'text-green-600',
      Icon: CheckCircle,
    },
  }

  const style = typeStyles[notification.type]
  const Icon = style.Icon

  return (
    <div className={`rounded-lg border ${style.bg} ${style.border} p-4 mb-4`}>
      <div className="flex items-start gap-3">
        <Icon className={`h-5 w-5 mt-0.5 flex-shrink-0 ${style.icon}`} />
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
            <X className="h-4 w-4" />
          </button>
        )}
      </div>
    </div>
  )
}
