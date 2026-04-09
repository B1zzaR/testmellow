import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '../../api/admin'
import type { SystemNotification, NotificationType } from '../../api/types'
import { Card } from '../ui/Card'
import { Button } from '../ui/Button'
import { Input } from '../ui/Input'
import { Modal } from '../ui/Modal'
import { Spinner } from '../ui/Spinner'
import { AlertTriangle, AlertCircle, Info, CheckCircle, Edit2, Trash2, Plus } from 'lucide-react'

const notificationTypeIcons = {
  warning: AlertTriangle,
  error: AlertCircle,
  info: Info,
  success: CheckCircle,
}

const notificationTypeColors = {
  warning: 'bg-yellow-50 text-yellow-800 border-yellow-200',
  error: 'bg-red-50 text-red-800 border-red-200',
  info: 'bg-blue-50 text-blue-800 border-blue-200',
  success: 'bg-green-50 text-green-800 border-green-200',
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
        <h1 className="text-2xl font-bold">System Notifications</h1>
        <Button
          onClick={() => {
            setEditingId(null)
            setFormData({ type: 'info', title: '', message: '' })
            setShowForm(true)
          }}
          className="gap-2"
        >
          <Plus className="h-4 w-4" />
          Create Notification
        </Button>
      </div>

      {/* Form Modal */}
      <Modal isOpen={showForm} onClose={handleCancel} title={editingId ? 'Edit Notification' : 'Create Notification'}>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2">Type</label>
            <select
              value={formData.type}
              onChange={(e) => setFormData({ ...formData, type: e.target.value as NotificationType })}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            >
              <option value="info">Info</option>
              <option value="success">Success</option>
              <option value="warning">Warning</option>
              <option value="error">Error</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Title</label>
            <Input
              value={formData.title}
              onChange={(e) => setFormData({ ...formData, title: e.target.value })}
              placeholder="Notification title"
              maxLength={100}
            />
          </div>

          <div>
            <label className="block text-sm font-medium mb-2">Message</label>
            <textarea
              value={formData.message}
              onChange={(e) => setFormData({ ...formData, message: e.target.value })}
              placeholder="Notification message"
              maxLength={500}
              rows={4}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent font-mono text-sm"
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
                  Saving...
                </>
              ) : (
                'Save'
              )}
            </Button>
            <Button variant="secondary" onClick={handleCancel}>
              Cancel
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
          No notifications yet
        </Card>
      ) : (
        <div className="space-y-3">
          {notifications.map((notif) => {
            const Icon = notificationTypeIcons[notif.type]
            return (
              <Card key={notif.id} className={`p-4 border ${notificationTypeColors[notif.type]}`}>
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-start gap-3 flex-1 min-w-0">
                    <Icon className="h-5 w-5 mt-0.5 flex-shrink-0" />
                    <div className="flex-1 min-w-0">
                      <h3 className="font-semibold text-sm">{notif.title}</h3>
                      <p className="text-sm mt-1 break-words">{notif.message}</p>
                      <div className="flex items-center gap-2 mt-2 text-xs opacity-75">
                        <span>Status: {notif.is_active ? '✓ Active' : '○ Inactive'}</span>
                        <span>•</span>
                        <span>{new Date(notif.created_at).toLocaleDateString()}</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex flex-col gap-2 flex-shrink-0">
                    <button
                      onClick={() => toggleActiveMutation.mutate(notif)}
                      disabled={toggleActiveMutation.isPending}
                      className="px-2 py-1 text-xs rounded bg-gray-200 hover:bg-gray-300 transition disabled:opacity-50"
                    >
                      {notif.is_active ? 'Deactivate' : 'Activate'}
                    </button>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => handleEdit(notif)}
                      className="gap-1"
                    >
                      <Edit2 className="h-3 w-3" />
                      Edit
                    </Button>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => deleteMutation.mutate(notif.id)}
                      disabled={deleteMutation.isPending}
                      className="gap-1 text-red-600 hover:text-red-700"
                    >
                      <Trash2 className="h-3 w-3" />
                      Delete
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
