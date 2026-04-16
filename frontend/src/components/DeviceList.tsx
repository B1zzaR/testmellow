import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { devicesApi } from '@/api/devices'
import { shopApi } from '@/api/shop'
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

const PRICE_RUB = 40
const PRICE_YAD = 16
const MAX_EXTRA = 2

interface DeviceListProps {
  data: DeviceListResponse
  isTrial?: boolean
}

export function DeviceList({ data, isTrial = false }: DeviceListProps) {
  const { devices, count, limit, expansion } = data
  const queryClient = useQueryClient()
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const [showInactive, setShowInactive] = useState(false)
  const [confirmId, setConfirmId] = useState<string | null>(null)
  const [buyMethod, setBuyMethod] = useState<'yad' | 'money' | null>(null)

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

  const buyExpansionYADMutation = useMutation({
    mutationFn: () => shopApi.buyDeviceExpansion(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['devices'] })
      queryClient.invalidateQueries({ queryKey: ['balance'] })
      setSuccessMsg('Расширение устройств активировано!')
      setErrorMsg('')
      setBuyMethod(null)
    },
    onError: (e: Error) => {
      setErrorMsg(e.message)
      setSuccessMsg('')
      setBuyMethod(null)
    },
  })

  const buyExpansionMoneyMutation = useMutation({
    mutationFn: () => shopApi.buyDeviceExpansionMoney(window.location.href),
    onSuccess: (data) => {
      setBuyMethod(null)
      window.location.href = data.redirect_url
    },
    onError: (e: Error) => {
      setErrorMsg(e.message)
      setSuccessMsg('')
      setBuyMethod(null)
    },
  })

  const currentExtra = expansion?.extra_devices ?? 0
  const canBuyMore = currentExtra < MAX_EXTRA

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

      {/* Device Expansion Section */}
      <div className="mt-6 border-t border-gray-200 dark:border-surface-700 pt-5">
        <p className="text-sm font-semibold text-gray-900 dark:text-slate-100 mb-1">Расширение лимита устройств</p>
        {expansion ? (
          <div className="mb-3 flex items-center gap-3 rounded-xl border border-primary-500/30 bg-primary-500/5 px-4 py-3">
            <Icon name="shield" size={16} className="shrink-0 text-primary-500" />
            <div>
              <p className="text-sm text-primary-500 font-medium">
                +{expansion.extra_devices} устройств (до {4 + expansion.extra_devices})
              </p>
              <p className="text-xs text-gray-400 dark:text-slate-500">
                Действует до {formatDateTime(expansion.expires_at)}
              </p>
            </div>
          </div>
        ) : (
          <p className="text-xs text-gray-400 dark:text-slate-500 mb-3">
            Дополнительные устройства действуют до конца текущей подписки. Максимум +{MAX_EXTRA}.
          </p>
        )}

        {isTrial ? (
          <div className="flex items-center gap-3 rounded-xl border border-yellow-500/30 bg-yellow-500/5 px-4 py-3">
            <Icon name="lock" size={16} className="shrink-0 text-yellow-500" />
            <div>
              <p className="text-sm text-yellow-500 font-medium">Недоступно на пробной подписке</p>
              <p className="text-xs text-gray-400 dark:text-slate-500">
                Расширение устройств доступно только на платных тарифах. Оформите подписку, чтобы добавить устройства.
              </p>
            </div>
          </div>
        ) : canBuyMore ? (
          <div className="rounded-xl border border-gray-200 dark:border-surface-700 p-4">
            <p className="text-base font-bold text-gray-900 dark:text-slate-100">
              +1 устройство
            </p>
            <p className="text-xs text-gray-400 dark:text-slate-500">
              Действует до конца текущей подписки
            </p>
            <p className="mt-2 text-xl font-extrabold text-primary-500">
              {PRICE_RUB}₽
            </p>
            <div className="mt-3 flex gap-2">
              <Button
                className="flex-1"
                size="sm"
                onClick={() => setBuyMethod('money')}
              >
                Оплатить {PRICE_RUB}₽
              </Button>
              <Button
                className="flex-1"
                size="sm"
                variant="secondary"
                onClick={() => setBuyMethod('yad')}
              >
                Оплатить {PRICE_YAD} ЯД
              </Button>
            </div>
          </div>
        ) : expansion ? (
          <p className="text-xs text-gray-400 dark:text-slate-500">
            Достигнут максимум дополнительных устройств (+{MAX_EXTRA}).
          </p>
        ) : null}
      </div>

      {/* Buy expansion confirmation modal */}
      <Modal
        open={buyMethod !== null}
        onClose={() => setBuyMethod(null)}
        title="Расширение устройств"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setBuyMethod(null)}>
              Отмена
            </Button>
            <Button
              size="sm"
              loading={buyExpansionYADMutation.isPending || buyExpansionMoneyMutation.isPending}
              onClick={() => {
                if (buyMethod === 'yad') buyExpansionYADMutation.mutate()
                if (buyMethod === 'money') buyExpansionMoneyMutation.mutate()
              }}
            >
              Подтвердить
            </Button>
          </>
        }
      >
        <p className="text-sm text-gray-600 dark:text-slate-400">
          {expansion ? 'Добавить ещё' : 'Активировать'}{' '}
          <strong className="text-gray-900 dark:text-slate-100">+1 устройство</strong>{' '}
          (итого до {4 + currentExtra + 1}) до конца текущей подписки за{' '}
          <strong className="text-primary-500">
            {buyMethod === 'money' ? `${PRICE_RUB}₽` : `${PRICE_YAD} ЯД`}
          </strong>?
        </p>
        {buyMethod === 'money' && (
          <p className="mt-2 text-xs text-gray-400 dark:text-slate-600">
            Вы будете перенаправлены на страницу оплаты.
          </p>
        )}
        {buyMethod === 'yad' && (
          <p className="mt-2 text-xs text-gray-400 dark:text-slate-600">
            Сумма будет списана с баланса ЯД.
          </p>
        )}
      </Modal>
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
