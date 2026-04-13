import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { devicesApi } from '@/api/devices'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { Modal } from '@/components/ui/Modal'
import { Icon } from '@/components/ui/Icons'
import { formatDateTime } from '@/utils/formatters'
import type { Device, DeviceListResponse } from '@/api/types'

function timeUntilDeletion(canDeleteAfter: string): string | null {
  const diff = new Date(canDeleteAfter).getTime() - Date.now()
  if (diff <= 0) return null // already can delete
  const hours = Math.ceil(diff / (1000 * 60 * 60))
  if (hours >= 24) {
    const days = Math.ceil(hours / 24)
    return `${days} дн.`
  }
  return `${hours} ч.`
}

interface DeviceListProps {
  data: DeviceListResponse
}

export function DeviceList({ data }: DeviceListProps) {
  const { devices, count, limit } = data
  const queryClient = useQueryClient()
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const [showInactive, setShowInactive] = useState(false)
  const [confirmId, setConfirmId] = useState<string | null>(null)

  const disconnectMutation = useMutation({
    mutationFn: (id: string) => devicesApi.disconnect(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['devices'] })
      setSuccessMsg('Устройство отключено')
      setErrorMsg('')
      setConfirmId(null)
    },
    onError: (e: Error) => {
      setErrorMsg(e.message)
      setSuccessMsg('')
      setConfirmId(null)
    },
  })

  const activeDevices = devices.filter(d => d.is_active)
  const inactiveDevices = devices.filter(d => !d.is_active)
  const atLimit = count >= limit

  return (
    <Card
      title="Устройства"
      subtitle={`${count} / ${limit} активных`}
    >
      {successMsg && <Alert variant="success" message={successMsg} className="mb-4" />}
      {errorMsg && <Alert variant="error" message={errorMsg} className="mb-4" />}

      {/* Disconnect confirmation modal */}
      <Modal
        open={confirmId !== null}
        onClose={() => setConfirmId(null)}
        title="Отключить устройство"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setConfirmId(null)}>
              Отмена
            </Button>
            <Button
              variant="danger"
              size="sm"
              loading={disconnectMutation.isPending}
              onClick={() => confirmId && disconnectMutation.mutate(confirmId)}
            >
              Отключить
            </Button>
          </>
        }
      >
        <p className="text-sm text-gray-600 dark:text-slate-400">
          Вы уверены, что хотите отключить это устройство? Оно потеряет доступ к VPN.
        </p>
      </Modal>

      {atLimit && (
        <div className="mb-4 flex items-center gap-3 rounded-xl border border-yellow-500/30 bg-yellow-500/5 px-4 py-3">
          <Icon name="zap" size={16} className="shrink-0 text-yellow-500" />
          <p className="text-sm text-yellow-500">Достигнут лимит устройств. Отключите неактивное устройство, чтобы добавить новое.</p>
        </div>
      )}

      {devices.length === 0 ? (
        <p className="text-sm text-gray-400 dark:text-slate-500">Нет подключённых устройств</p>
      ) : (
        <div className="space-y-4">
          <p className="text-xs text-gray-400 dark:text-slate-600">
            Устройство можно удалить после 2 дней неактивности, чтобы привязать новое.
          </p>
          {/* Active devices */}
          {activeDevices.length > 0 && (
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-slate-600">Активные ({activeDevices.length})</p>
              {activeDevices.map((device) => (
                <DeviceRow
                  key={device.id}
                  device={device}
                  onDisconnect={() => setConfirmId(device.id)}
                  loading={disconnectMutation.isPending && disconnectMutation.variables === device.id}
                />
              ))}
            </div>
          )}

          {/* Inactive devices */}
          {inactiveDevices.length > 0 && (
            <div className="space-y-2">
              <button
                onClick={() => setShowInactive(!showInactive)}
                className="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-slate-600 hover:text-gray-600 dark:hover:text-slate-500 transition-colors flex items-center gap-1"
              >
                <Icon name={showInactive ? 'chevron-down' : 'chevron-right'} size={12} />
                Неактивные ({inactiveDevices.length})
              </button>
              {showInactive && (
                <div className="space-y-2">
                  {inactiveDevices.map((device) => (
                    <DeviceRow
                      key={device.id}
                      device={device}
                      onDisconnect={() => setConfirmId(device.id)}
                      loading={disconnectMutation.isPending && disconnectMutation.variables === device.id}
                    />
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </Card>
  )
}

interface DeviceRowProps {
  device: Device
  onDisconnect: () => void
  loading: boolean
}

function DeviceRow({ device, onDisconnect, loading }: DeviceRowProps) {
  const inactive = device.is_inactive
  const remaining = timeUntilDeletion(device.can_delete_after)

  return (
    <div
      className={[
        'flex items-center justify-between rounded-lg border px-4 py-3 transition-colors',
        inactive
          ? 'border-gray-200 dark:border-surface-600 bg-gray-50/50 dark:bg-surface-800/60'
          : 'border-gray-100 dark:border-surface-700 hover:bg-gray-50 dark:hover:bg-surface-800',
      ].join(' ')}
    >
      <div className="flex min-w-0 items-center gap-3">
        {/* Status dot */}
        <span
          className={[
            'h-2 w-2 shrink-0 rounded-full',
            inactive ? 'bg-gray-400 dark:bg-slate-600' : 'bg-primary-500',
          ].join(' ')}
        />

        <div className="min-w-0">
          <p className="truncate font-medium text-gray-800 dark:text-slate-200">
            {device.device_name}
          </p>
          <p className="mt-0.5 text-xs text-gray-400 dark:text-slate-500">
            Последняя активность: {formatDateTime(device.last_active)}
          </p>
          {!inactive && remaining && (
            <p className="mt-0.5 text-xs text-gray-400 dark:text-slate-600">
              Удаление доступно через {remaining}
            </p>
          )}
          {inactive && (
            <p className="mt-0.5 text-xs text-primary-500">
              Можно удалить
            </p>
          )}
        </div>
      </div>

      <div className="ml-4 flex shrink-0 items-center gap-3">
        <span
          className={[
            'rounded-full px-2.5 py-0.5 text-[11px] font-semibold',
            inactive
              ? 'bg-gray-100 dark:bg-surface-700 text-gray-500 dark:text-slate-400'
              : 'bg-primary-500/10 text-primary-500',
          ].join(' ')}
        >
          {inactive ? 'Неактивно' : 'Активно'}
        </span>

        {inactive && (
          <Button
            variant="danger"
            size="sm"
            loading={loading}
            onClick={onDisconnect}
          >
            Отключить
          </Button>
        )}
      </div>
    </div>
  )
}
