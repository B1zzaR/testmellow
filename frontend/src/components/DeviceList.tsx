import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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
  if (diff <= 0) return null
  const hours = Math.ceil(diff / (1000 * 60 * 60))
  if (hours >= 24) {
    const days = Math.ceil(hours / 24)
    return `${days} дн.`
  }
  return `${hours} ч.`
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' })
}

interface DeviceListProps {
  data: DeviceListResponse
  isTrial?: boolean
}

export function DeviceList({ data }: DeviceListProps) {
  const { devices, count, limit, expansion } = data
  const queryClient = useQueryClient()
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const [showInactive, setShowInactive] = useState(false)
  const [confirmId, setConfirmId] = useState<string | null>(null)
  const [showExpansionPanel, setShowExpansionPanel] = useState(false)

  const canExpand = !expansion || expansion.extra_devices < 2

  const { data: quote } = useQuery({
    queryKey: ['device-expansion-quote'],
    queryFn: shopApi.getDeviceExpansionQuote,
    enabled: showExpansionPanel,
  })

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

  const buyYADMutation = useMutation({
    mutationFn: (qty: 1 | 2) => shopApi.buyDeviceExpansion(qty),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['devices'] })
      queryClient.invalidateQueries({ queryKey: ['device-expansion-quote'] })
      setErrorMsg('')
      setShowExpansionPanel(false)
    },
    onError: (e: Error) => {
      setErrorMsg(e.message)
      setSuccessMsg('')
    },
  })

  const buyMoneyMutation = useMutation({
    mutationFn: (qty: 1 | 2) =>
      shopApi.buyDeviceExpansionMoney(qty, window.location.href),
    onSuccess: (res) => {
      window.location.href = res.redirect_url
    },
    onError: (e: Error) => {
      setErrorMsg(e.message)
      setSuccessMsg('')
    },
  })

  const activeDevices = devices.filter(d => d.is_active && !d.is_blocked)
  const blockedDevices = devices.filter(d => d.is_blocked)
  const inactiveDevices = devices.filter(d => !d.is_active && !d.is_blocked)
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

      {/* Active expansion banner */}
      {expansion && (
        <div className="mb-4 flex items-center gap-3 rounded-xl border border-primary-500/30 bg-primary-500/5 px-4 py-3">
          <Icon name="check-circle" size={16} className="shrink-0 text-primary-500" />
          <p className="text-sm text-primary-600 dark:text-primary-400">
            ✅ +{expansion.extra_devices} устройств расширено до {formatDate(expansion.expires_at)}
          </p>
        </div>
      )}

      {atLimit && !expansion && (
        <div className="mb-4 flex items-center gap-3 rounded-xl border border-yellow-500/30 bg-yellow-500/5 px-4 py-3">
          <Icon name="zap" size={16} className="shrink-0 text-yellow-500" />
          <p className="text-sm text-yellow-500">Достигнут лимит устройств. Отключите неактивное устройство или расширьте лимит.</p>
        </div>
      )}

      {atLimit && expansion && (
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
            Устройство можно удалить после {3} дней неактивности, чтобы привязать новое.
          </p>
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

          {blockedDevices.length > 0 && (
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wider text-red-500 dark:text-red-400">Заблокированы ({blockedDevices.length})</p>
              <div className="rounded-xl border border-red-500/20 bg-red-500/5 px-4 py-2.5 mb-2">
                <p className="text-xs text-red-500 dark:text-red-400">
                  Эти устройства превышают текущий лимит. Отключите одно из активных.
                </p>
              </div>
              {blockedDevices.map((device) => (
                <DeviceRow
                  key={device.id}
                  device={device}
                  onDisconnect={() => setConfirmId(device.id)}
                  loading={disconnectMutation.isPending && disconnectMutation.variables === device.id}
                />
              ))}
            </div>
          )}

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

      {/* Device expansion section */}
      {canExpand && (
        <div className="mt-5 border-t border-gray-100 dark:border-surface-700 pt-4">
          {!showExpansionPanel ? (
            <button
              onClick={() => setShowExpansionPanel(true)}
              className="flex items-center gap-2 text-sm text-primary-500 hover:text-primary-600 transition-colors"
            >
              <Icon name="check-circle" size={16} />
              {expansion ? 'Апгрейд до +2 устройств' : '🔓 Расширить устройства'}
            </button>
          ) : (
            <div className="space-y-4">
              {/* Header */}
              <div className="flex items-start justify-between">
                <div>
                  <p className="text-sm font-semibold text-gray-800 dark:text-slate-200">🔓 Расширение устройств</p>
                  <p className="text-xs text-gray-400 dark:text-slate-500 mt-0.5">
                    Действует до конца текущей подписки
                  </p>
                </div>
                {/* Tier badge */}
                {quote && quote.tier_label && (
                  <TierBadge label={quote.tier_label} daysRemaining={quote.days_remaining} />
                )}
              </div>

              {/* Days remaining hint */}
              {quote && quote.days_remaining > 0 && (
                <div className="flex items-center gap-2 rounded-lg bg-gray-50 dark:bg-surface-800 px-3 py-2">
                  <span className="text-xs text-gray-500 dark:text-slate-400">
                    📅 Осталось <span className="font-semibold text-gray-700 dark:text-slate-300">{quote.days_remaining} дн.</span> подписки
                    {quote.tier_label
                      ? ` — цена снижена`
                      : ` — стандартная цена`}
                  </span>
                </div>
              )}

              <div className="space-y-2">
                {(!expansion || expansion.extra_devices < 1) && quote && (
                  <ExpansionOption
                    label="+1 устройство"
                    yadPrice={quote.qty1.yad}
                    rublesPrice={quote.qty1.rubles}
                    unitRubles={quote.unit_rubles}
                    showDiscount2={false}
                    onBuyYAD={() => buyYADMutation.mutate(1)}
                    onBuyMoney={() => buyMoneyMutation.mutate(1)}
                    loading={buyYADMutation.isPending || buyMoneyMutation.isPending}
                  />
                )}
                {(!expansion || expansion.extra_devices < 2) && quote && (
                  <ExpansionOption
                    label={expansion?.extra_devices === 1 ? 'Апгрейд до +2 устройств' : '+2 устройства'}
                    yadPrice={quote.qty2.yad}
                    rublesPrice={quote.qty2.rubles}
                    unitRubles={quote.unit_rubles}
                    showDiscount2
                    onBuyYAD={() => buyYADMutation.mutate(2)}
                    onBuyMoney={() => buyMoneyMutation.mutate(2)}
                    loading={buyYADMutation.isPending || buyMoneyMutation.isPending}
                  />
                )}
                {!quote && (
                  <p className="text-xs text-gray-400 dark:text-slate-500 py-2">Загрузка цен…</p>
                )}
              </div>
              <button
                onClick={() => setShowExpansionPanel(false)}
                className="text-xs text-gray-400 hover:text-gray-600 dark:hover:text-slate-400 transition-colors"
              >
                Скрыть
              </button>
            </div>
          )}
        </div>
      )}
    </Card>
  )
}

// ─── Tier badge ───────────────────────────────────────────────────────────────

interface TierBadgeProps {
  label: string
  daysRemaining: number
}

function TierBadge({ label, daysRemaining }: TierBadgeProps) {
  const isHot = daysRemaining < 8
  return (
    <span
      className={[
        'inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-bold tracking-wide',
        isHot
          ? 'bg-red-500/10 text-red-500 dark:bg-red-500/20 dark:text-red-400'
          : 'bg-orange-500/10 text-orange-500 dark:bg-orange-400/20 dark:text-orange-400',
      ].join(' ')}
    >
      {isHot ? '⚡' : '🔥'} {label}
    </span>
  )
}

// ─── Expansion option row ─────────────────────────────────────────────────────

interface ExpansionOptionProps {
  label: string
  yadPrice: number
  rublesPrice: number
  unitRubles: number
  showDiscount2: boolean
  onBuyYAD: () => void
  onBuyMoney: () => void
  loading: boolean
}

function ExpansionOption({
  label,
  yadPrice,
  rublesPrice,
  unitRubles,
  showDiscount2,
  onBuyYAD,
  onBuyMoney,
  loading,
}: ExpansionOptionProps) {
  // Savings from the 10% second-slot discount
  const savedRubles = showDiscount2 ? Math.round(unitRubles * 0.1) : 0

  return (
    <div className="rounded-xl border border-gray-100 dark:border-surface-700 px-4 py-3 space-y-2">
      {/* Top row: label + discount badge */}
      <div className="flex items-center justify-between">
        <p className="text-sm font-semibold text-gray-800 dark:text-slate-200">{label}</p>
        {showDiscount2 && (
          <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2.5 py-0.5 text-[11px] font-bold text-emerald-600 dark:text-emerald-400">
            💥 −10% на 2-й слот
          </span>
        )}
      </div>

      {/* Price row */}
      <div className="flex items-end justify-between">
        <div>
          <p className="text-base font-bold text-gray-900 dark:text-slate-100">
            {yadPrice} ЯД
            <span className="mx-1.5 text-gray-300 dark:text-slate-600 font-normal">/</span>
            {rublesPrice} ₽
          </p>
          {showDiscount2 && savedRubles > 0 && (
            <p className="text-[11px] text-emerald-500 dark:text-emerald-400 mt-0.5">
              Экономия {savedRubles} ₽ против двух отдельных
            </p>
          )}
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" loading={loading} onClick={onBuyYAD}>
            За ЯД
          </Button>
          <Button variant="primary" size="sm" loading={loading} onClick={onBuyMoney}>
            За рубли
          </Button>
        </div>
      </div>
    </div>
  )
}

interface DeviceRowProps {
  device: Device
  onDisconnect: () => void
  loading: boolean
}

function DeviceRow({ device, onDisconnect, loading }: DeviceRowProps) {
  const inactive = device.is_inactive
  const blocked = device.is_blocked
  const remaining = timeUntilDeletion(device.can_delete_after)

  return (
    <div
      className={[
        'flex items-center justify-between rounded-lg border px-4 py-3 transition-colors',
        blocked
          ? 'border-red-500/30 bg-red-50/50 dark:bg-red-900/10 opacity-75'
          : inactive
            ? 'border-gray-200 dark:border-surface-600 bg-gray-50/50 dark:bg-surface-800/60'
            : 'border-gray-100 dark:border-surface-700 hover:bg-gray-50 dark:hover:bg-surface-800',
      ].join(' ')}
    >
      <div className="flex min-w-0 items-center gap-3">
        <span
          className={[
            'h-2 w-2 shrink-0 rounded-full',
            blocked
              ? 'bg-red-500'
              : inactive ? 'bg-gray-400 dark:bg-slate-600' : 'bg-primary-500',
          ].join(' ')}
        />

        <div className="min-w-0">
          <p className="truncate font-medium text-gray-800 dark:text-slate-200">
            {device.device_name}
          </p>
          <p className="mt-0.5 text-xs text-gray-400 dark:text-slate-500">
            Последняя активность: {formatDateTime(device.last_active)}
          </p>
          {blocked && (
            <p className="mt-0.5 text-xs text-red-500">
              Заблокировано — превышен лимит устройств
            </p>
          )}
          {!blocked && !inactive && remaining && (
            <p className="mt-0.5 text-xs text-gray-400 dark:text-slate-600">
              Удаление доступно через {remaining}
            </p>
          )}
          {!blocked && inactive && (
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
            blocked
              ? 'bg-red-500/10 text-red-500'
              : inactive
                ? 'bg-gray-100 dark:bg-surface-700 text-gray-500 dark:text-slate-400'
                : 'bg-primary-500/10 text-primary-500',
          ].join(' ')}
        >
          {blocked ? 'Заблокировано' : inactive ? 'Неактивно' : 'Активно'}
        </span>

        {(inactive || blocked) && (
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
