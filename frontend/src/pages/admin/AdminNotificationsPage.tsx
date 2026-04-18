import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import type { SystemNotification, NotificationType } from '@/api/types'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Modal } from '@/components/ui/Modal'
import { Spinner } from '@/components/ui/Spinner'
import { Icon } from '@/components/ui/Icons'

const notificationTypeIcons = {
  warning: 'tag' as const,
  error: 'x-circle' as const,
  info: 'message' as const,
  success: 'check-circle' as const,
}

const notificationTypeColors = {
  warning: 'bg-yellow-50 dark:bg-yellow-950/30 text-yellow-800 dark:text-yellow-300 border-yellow-200 dark:border-yellow-700',
  error: 'bg-red-50 dark:bg-red-950/30 text-red-800 dark:text-red-300 border-red-200 dark:border-red-700',
  info: 'bg-blue-50 dark:bg-blue-950/30 text-blue-800 dark:text-blue-300 border-blue-200 dark:border-blue-700',
  success: 'bg-green-50 dark:bg-green-950/30 text-green-800 dark:text-green-300 border-green-200 dark:border-green-700',
}

interface NotificationFormData {
  type: NotificationType
  title: string
  message: string
}

export function AdminNotificationsPage() {
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [formData, setFormData] = useState<NotificationFormData>({
    type: 'info',
    title: '',
    message: '',
  })

  // Fetch notifications
  const { data, isLoading } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => adminApi.listNotifications({ limit: 50 }),
  })

  // Create notification
  const createMutation = useMutation({
    mutationFn: () => adminApi.createNotification(formData),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      setShowForm(false)
      setFormData({ type: 'info', title: '', message: '' })
    },
  })

  // Update notification
  const updateMutation = useMutation({
    mutationFn: () => {
      if (!editingId) throw new Error('No notification selected')
      return adminApi.updateNotification(editingId, formData)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      setEditingId(null)
      setFormData({ type: 'info', title: '', message: '' })
    },
  })

  // Delete notification
  const deleteMutation = useMutation({
    mutationFn: (id: string) => adminApi.deleteNotification(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
    },
  })

  // Toggle active status
  const toggleActiveMutation = useMutation({
    mutationFn: (notif: SystemNotification) =>
      adminApi.updateNotification(notif.id, { is_active: !notif.is_active }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
    },
  })

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!formData.title || !formData.message) return

    if (editingId) {
      await updateMutation.mutateAsync()
    } else {
      await createMutation.mutateAsync()
    }
  }

  const handleEdit = (notif: SystemNotification) => {
    setEditingId(notif.id)
    setFormData({
      type: notif.type,
      title: notif.title,
      message: notif.message,
    })
    setShowForm(true)
  }

  const handleCancel = () => {
    setEditingId(null)
    setShowForm(false)
    setFormData({ type: 'info', title: '', message: '' })
  }

  const notifications = data?.notifications || []

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold">Системные уведомления</h1>
        <Button
          onClick={() => {
            setEditingId(null)
            setFormData({ type: 'info', title: '', message: '' })
            setShowForm(true)
          }}
          className="gap-2"
        >
          <Icon name="bell" size={16} />
          Создать уведомление
        </Button>
      </div>

      {/* Form Modal */}
      <Modal open={showForm} onClose={handleCancel} title={editingId ? 'Редактировать уведомление' : 'Создать уведомление'}>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">Тип</label>
            <select
              value={formData.type}
              onChange={(e) => setFormData({ ...formData, type: e.target.value as NotificationType })}
              className="w-full px-3 py-2 border border-gray-300 dark:border-surface-600 rounded-lg bg-white dark:bg-surface-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            >
              <option value="info">Информация</option>
              <option value="success">Успех</option>
              <option value="warning">Предупреждение</option>
              <option value="error">Ошибка</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Заголовок</label>
            <Input
              value={formData.title}
              onChange={(e) => setFormData({ ...formData, title: e.target.value })}
              placeholder="Заголовок уведомления"
              maxLength={100}
            />
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Сообщение</label>
            <textarea
              value={formData.message}
              onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setFormData({ ...formData, message: e.target.value })}
              placeholder="Текст уведомления"
              maxLength={500}
              rows={4}
              className="w-full px-3 py-2 border border-gray-300 dark:border-surface-600 rounded-lg bg-white dark:bg-surface-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-transparent font-mono text-sm"
            />
            <p className="text-xs text-gray-500 mt-1">{formData.message.length}/500</p>
          </div>

          <div className="flex gap-2 pt-4">
            <Button
              type="submit"
              disabled={createMutation.isPending || updateMutation.isPending}
            >
              {createMutation.isPending || updateMutation.isPending ? (
                <>
                  <Spinner className="h-4 w-4" />
                  Сохранение...
                </>
              ) : (
                'Сохранить'
              )}
            </Button>
            <Button variant="secondary" onClick={handleCancel}>
              Отмена
            </Button>
          </div>
        </form>
      </Modal>

      {/* Notifications List */}
      {isLoading ? (
        <div className="flex justify-center py-8">
          <Spinner />
        </div>
      ) : notifications.length === 0 ? (
        <Card className="text-center py-8 text-gray-500">
          Уведомлений пока нет
        </Card>
      ) : (
        <div className="space-y-3">
          {notifications.map((notif) => {
            const iconName = notificationTypeIcons[notif.type]
            return (
              <Card key={notif.id} className={`p-4 border ${notificationTypeColors[notif.type]}`}>
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-start gap-3 flex-1 min-w-0">
                    <Icon name={iconName} size={20} className="mt-0.5 flex-shrink-0" />
                    <div className="flex-1 min-w-0">
                      <h3 className="font-semibold text-sm">{notif.title}</h3>
                      <p className="text-sm mt-1 break-words">{notif.message}</p>
                      <div className="flex items-center gap-2 mt-2 text-xs opacity-75">
                        <span>Статус: {notif.is_active ? '✓ Активно' : '○ Неактивно'}</span>
                        <span>•</span>
                        <span>{new Date(notif.created_at).toLocaleDateString()}</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex flex-col gap-2 flex-shrink-0">
                    <button
                      onClick={() => toggleActiveMutation.mutate(notif)}
                      disabled={toggleActiveMutation.isPending}
                      className="px-2 py-1 text-xs rounded bg-gray-200 dark:bg-surface-700 hover:bg-gray-300 dark:hover:bg-surface-600 transition disabled:opacity-50 text-gray-900 dark:text-gray-100"
                    >
                      {notif.is_active ? 'Деактивировать' : 'Активировать'}
                    </button>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => handleEdit(notif)}
                      className="gap-1"
                    >
                      <Icon name="settings" size={12} />
                      Изменить
                    </Button>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => deleteMutation.mutate(notif.id)}
                      disabled={deleteMutation.isPending}
                      className="gap-1 text-red-600 hover:text-red-700"
                    >
                      <Icon name="close" size={12} />
                      Удалить
                    </Button>
                  </div>
                </div>
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
