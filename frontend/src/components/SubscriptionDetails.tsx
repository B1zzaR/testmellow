import { formatDate, planLabel } from '@/utils/formatters'
import type { Subscription } from '@/api/types'

interface SubscriptionDetailsProps {
  allSubscriptions: Subscription[]
  totalDays: number
}

export function SubscriptionDetails({ allSubscriptions, totalDays }: SubscriptionDetailsProps) {
  const activeSub = allSubscriptions.find(s => s.status === 'active' || s.status === 'trial')
  if (!activeSub) return null

  return (
    <div className="mt-5 border-t border-gray-100 dark:border-surface-700 pt-4">
      <p className="text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-slate-600 mb-3">
        Детали подписки
      </p>

      <div className="rounded-lg border border-gray-200 dark:border-surface-600 divide-y divide-gray-100 dark:divide-surface-700">
        <Row label="Тариф" value={planLabel(activeSub.plan)} />
        <Row label="Начало" value={formatDate(activeSub.starts_at)} />
        <Row label="Конец" value={formatDate(activeSub.expires_at)} />
        <Row label="Осталось дней" value={`${totalDays} дн.`} />
        <Row label="Устройства" value={<span>4<span className="ml-1 text-gray-400 dark:text-slate-500 text-xs">/ макс 4</span></span>} />
      </div>
    </div>
  )
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between px-3 py-2">
      <span className="text-xs text-gray-400 dark:text-slate-500">{label}</span>
      <span className="text-xs font-medium text-gray-900 dark:text-slate-100">{value}</span>
    </div>
  )
}
