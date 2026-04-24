import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { subscriptionsApi } from '@/api/subscriptions'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { Icon } from '@/components/ui/Icons'
import { formatDate, planLabel } from '@/utils/formatters'
import type { Subscription, SubscriptionPeriod } from '@/api/types'

interface SubscriptionDetailsProps {
  subscription: Subscription
  totalDays: number
}

export function SubscriptionDetails({ subscription, totalDays }: SubscriptionDetailsProps) {
  const [isExpanded, setIsExpanded] = useState(false)
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const queryClient = useQueryClient()

  const { data: periodsData, isLoading } = useQuery({
    queryKey: ['subscription-periods', subscription.id],
    queryFn: () => subscriptionsApi.getPeriods(subscription.id),
    enabled: isExpanded,
  })

  const buyDevicesMutation = useMutation({
    mutationFn: ({ periodId, extraDevices }: { periodId: string; extraDevices: number }) =>
      subscriptionsApi.buyPeriodDevices(periodId, extraDevices),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['subscription-periods', subscription.id] })
      queryClient.invalidateQueries({ queryKey: ['devices'] })
      setSuccessMsg('Устройства успешно добавлены к периоду!')
      setErrorMsg('')
    },
    onError: (e: Error) => {
      setErrorMsg(e.message)
      setSuccessMsg('')
    },
  })

  const periods = periodsData?.periods ?? []

  const handleBuyDevices = (periodId: string) => {
    buyDevicesMutation.mutate({ periodId, extraDevices: 1 })
  }

  return (
    <div className="mt-5 border-t border-gray-100 dark:border-surface-700 pt-4">
      {successMsg && <Alert variant="success" message={successMsg} className="mb-3" />}
      {errorMsg && <Alert variant="error" message={errorMsg} className="mb-3" />}

      <div className="flex items-center justify-between">
        <div>
          <p className="text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-slate-600">
            Детализация подписки
          </p>
          <p className="mt-1 text-sm font-semibold text-gray-900 dark:text-slate-100">
            Всего дней: {totalDays}
          </p>
        </div>
        <button
          onClick={() => setIsExpanded(!isExpanded)}
          className="flex items-center gap-2 rounded-lg border border-gray-200 dark:border-surface-600 bg-white dark:bg-surface-700 px-3 py-2 text-xs font-medium text-gray-700 dark:text-slate-300 hover:bg-gray-50 dark:hover:bg-surface-600 transition-all active:scale-95"
        >
          <Icon 
            name={isExpanded ? 'chevron-down' : 'chevron-right'} 
            size={14} 
            className="transition-transform"
          />
          <span>{isExpanded ? 'Скрыть' : 'Подробнее'}</span>
        </button>
      </div>

      {isExpanded && (
        <div className="mt-4 space-y-2 animate-expand overflow-hidden">
          {isLoading ? (
            <div className="flex items-center justify-center py-4">
              <div className="h-5 w-5 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" />
            </div>
          ) : periods.length === 0 ? (
            <p className="text-sm text-gray-400 dark:text-slate-500 py-2">
              Нет данных о периодах подписки
            </p>
          ) : (
            periods.map((period, index) => (
              <PeriodCard
                key={period.id}
                period={period}
                index={index}
                onBuyDevices={handleBuyDevices}
                isLoading={buyDevicesMutation.isPending}
              />
            ))
          )}
        </div>
      )}
    </div>
  )
}

interface PeriodCardProps {
  period: SubscriptionPeriod
  index: number
  onBuyDevices: (periodId: string) => void
  isLoading: boolean
}

function PeriodCard({ period, index, onBuyDevices, isLoading }: PeriodCardProps) {
  const isActive = period.status === 'active'
  const isQueued = period.status === 'queued'

  return (
    <div
      className={[
        'rounded-lg border p-4 transition-all',
        isActive
          ? 'border-primary-500/30 bg-primary-500/5 dark:bg-primary-500/10'
          : isQueued
            ? 'border-yellow-500/30 bg-yellow-500/5 dark:bg-yellow-500/10'
            : 'border-gray-200 dark:border-surface-600 bg-gray-50 dark:bg-surface-800',
      ].join(' ')}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-2">
            <span
              className={[
                'h-2 w-2 shrink-0 rounded-full',
                isActive
                  ? 'bg-primary-500'
                  : isQueued
                    ? 'bg-yellow-500'
                    : 'bg-gray-400 dark:bg-slate-600',
              ].join(' ')}
            />
            <p className="text-sm font-semibold text-gray-900 dark:text-slate-100">
              Период {index + 1}: {planLabel(period.plan)}
            </p>
            <span
              className={[
                'rounded-full px-2 py-0.5 text-[10px] font-semibold',
                isActive
                  ? 'bg-primary-500/10 text-primary-500'
                  : isQueued
                    ? 'bg-yellow-500/10 text-yellow-500'
                    : 'bg-gray-100 dark:bg-surface-700 text-gray-500 dark:text-slate-400',
              ].join(' ')}
            >
              {isActive ? 'Активна' : isQueued ? 'В очереди' : 'Истекла'}
            </span>
          </div>

          <div className="grid grid-cols-2 gap-3 text-xs">
            <div>
              <p className="text-gray-400 dark:text-slate-600">Длительность</p>
              <p className="mt-0.5 font-medium text-gray-900 dark:text-slate-100">
                {period.duration_days} дней
              </p>
            </div>
            <div>
              <p className="text-gray-400 dark:text-slate-600">Устройств</p>
              <p className="mt-0.5 font-medium text-gray-900 dark:text-slate-100">
                {period.device_slots}
              </p>
            </div>
            <div>
              <p className="text-gray-400 dark:text-slate-600">Начало</p>
              <p className="mt-0.5 font-medium text-gray-900 dark:text-slate-100">
                {formatDate(period.starts_at)}
              </p>
            </div>
            <div>
              <p className="text-gray-400 dark:text-slate-600">Окончание</p>
              <p className="mt-0.5 font-medium text-gray-900 dark:text-slate-100">
                {formatDate(period.expires_at)}
              </p>
            </div>
          </div>
        </div>

        {(isActive || isQueued) && (
          <Button
            size="sm"
            variant="secondary"
            onClick={() => onBuyDevices(period.id)}
            loading={isLoading}
            className="shrink-0"
          >
            <Icon name="zap" size={12} />
            <span className="hidden sm:inline">Добавить устройства</span>
            <span className="sm:hidden">+</span>
          </Button>
        )}
      </div>

      <div className="mt-3 flex items-center gap-2 rounded-lg border border-blue-500/20 bg-blue-500/5 px-3 py-2">
        <Icon name="info" size={12} className="shrink-0 text-blue-500" />
        <p className="text-[11px] text-blue-600 dark:text-blue-400">
          Дополнительные устройства действуют только в рамках этого периода
        </p>
      </div>
    </div>
  )
}
