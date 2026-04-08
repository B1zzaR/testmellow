import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { paymentsApi } from '@/api/payments'
import { Card } from '@/components/ui/Card'
import { PageSpinner } from '@/components/ui/Spinner'
import { Icon } from '@/components/ui/Icons'
import { formatDateTime, formatRubles, planLabel } from '@/utils/formatters'
import type { PaymentStatus, SubscriptionPlan } from '@/api/types'

const statusLabel: Record<PaymentStatus, string> = {
  PENDING: 'Ожидает',
  CONFIRMED: 'Оплачен',
  CANCELED: 'Отменён',
  CHARGEBACKED: 'Возврат',
  EXPIRED: 'Истёк',
}

const statusColor: Record<PaymentStatus, string> = {
  PENDING: 'text-yellow-500',
  CONFIRMED: 'text-primary-500',
  CANCELED: 'text-gray-400 dark:text-slate-600',
  CHARGEBACKED: 'text-red-500',
  EXPIRED: 'text-gray-400 dark:text-slate-600',
}

const PER_PAGE = 10

export function PaymentHistoryPage() {
  const [page, setPage] = useState(1)

  const { data, isLoading } = useQuery({
    queryKey: ['payment-history', page],
    queryFn: () => paymentsApi.listHistory(page, PER_PAGE),
    placeholderData: (prev) => prev,
  })

  const payments = data?.payments ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE))

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">История платежей</h1>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">Все ваши транзакции</p>
      </div>

      <Card>
        {isLoading ? (
          <PageSpinner />
        ) : payments.length === 0 ? (
          <div className="py-10 text-center">
            <Icon name="coins" size={28} className="mx-auto mb-3 text-gray-300 dark:text-slate-700" />
            <p className="text-sm text-gray-400 dark:text-slate-600">Платежей пока нет</p>
          </div>
        ) : (
          <div className="divide-y divide-gray-100 dark:divide-surface-700">
            {payments.map((p) => (
              <div key={p.id} className="flex items-center justify-between px-1 py-3.5 hover:bg-gray-50 dark:hover:bg-surface-800/50 rounded-lg transition-colors">
                <div className="space-y-0.5">
                  <p className="text-sm font-medium text-gray-800 dark:text-slate-200">
                    {planLabel(p.plan as SubscriptionPlan)}
                  </p>
                  <p className="text-xs text-gray-400 dark:text-slate-600">{formatDateTime(p.created_at)}</p>
                  <p className="font-mono text-xs text-gray-300 dark:text-slate-700">#{p.id.slice(0, 8)}</p>
                </div>
                <div className="text-right space-y-0.5">
                  <p className="text-sm font-bold text-gray-900 dark:text-slate-100">
                    {p.amount_kopecks === 0 ? 'Бесплатно' : formatRubles(p.amount_kopecks)}
                  </p>
                  <p className={`text-xs font-medium ${statusColor[p.status]}`}>
                    {statusLabel[p.status]}
                  </p>
                </div>
              </div>
            ))}
          </div>
        )}

        {totalPages > 1 && (
          <div className="mt-4 flex items-center justify-between border-t border-gray-100 dark:border-surface-700 pt-4">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
              className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm text-gray-600 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-surface-800 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <Icon name="back" size={14} />
              Назад
            </button>
            <span className="text-xs text-gray-400 dark:text-slate-600">
              {page} / {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page === totalPages}
              className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm text-gray-600 dark:text-slate-400 hover:bg-gray-100 dark:hover:bg-surface-800 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              Далее
              <Icon name="external" size={14} className="rotate-90" />
            </button>
          </div>
        )}
      </Card>
    </div>
  )
}
