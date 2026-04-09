import { useQuery } from '@tanstack/react-query'
import { balanceApi } from '@/api/profile'
import { Card, StatCard } from '@/components/ui/Card'
import { PageSpinner } from '@/components/ui/Spinner'
import { Icon } from '@/components/ui/Icons'
import { formatDateTime, formatYAD } from '@/utils/formatters'
import type { YADTxType } from '@/api/types'

const txTypeLabel: Record<YADTxType, string> = {
  referral_reward: 'Реферальное вознаграждение',
  bonus: 'Бонус',
  spent: 'Списание',
  promo: 'Промокод',
  trial: 'Пробный период',
}

export function BalancePage() {
  const { data: balance, isLoading: balanceLoading } = useQuery({
    queryKey: ['balance'],
    queryFn: balanceApi.getBalance,
  })
  const { data: history, isLoading: histLoading } = useQuery({
    queryKey: ['balance-history'],
    queryFn: balanceApi.getHistory,
  })

  if (balanceLoading) return <PageSpinner />

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">Баланс ЯДА</h1>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">Внутренняя валюта платформы</p>
      </div>

      <div className="grid gap-4 grid-cols-2 sm:grid-cols-3">
        <StatCard
          label="Текущий баланс"
          value={formatYAD(balance?.yad_balance ?? 0)}
          icon={<Icon name="skull" size={28} />}
          accent
        />
        <StatCard
          label="Курс обмена"
          value="1 ЯД = 2.50 ₽"
          sub="Фиксированный"
          icon={<Icon name="refresh" size={28} />}
        />
        <StatCard
          label="В рублях"
          value={`${(balance?.yad_ruble_value ?? 0).toFixed(2)} ₽`}
          sub="Эквивалент"
          icon={<Icon name="gem" size={28} />}
        />
      </div>

      <Card title="История транзакций">
        {histLoading ? (
          <PageSpinner />
        ) : !history?.transactions?.length ? (
          <div className="py-8 text-center">
            <Icon name="coins" size={28} className="mx-auto mb-2 text-gray-300 dark:text-slate-700" />
            <p className="text-sm text-gray-400 dark:text-slate-600">Транзакций пока нет</p>
          </div>
        ) : (
          <div className="space-y-1">
            {history.transactions.map((tx) => (
              <div
                key={tx.id}
                className="flex items-center justify-between rounded-lg px-4 py-3 hover:bg-gray-50 dark:hover:bg-surface-800 transition-colors"
              >
                <div>
                  <p className="text-sm font-medium text-gray-800 dark:text-slate-200">
                    {txTypeLabel[tx.tx_type] ?? tx.tx_type}
                  </p>
                  {tx.note && <p className="text-xs text-gray-400 dark:text-slate-600">{tx.note}</p>}
                  <p className="text-xs text-gray-400 dark:text-slate-600">{formatDateTime(tx.created_at)}</p>
                </div>
                <div className="text-right">
                  <p className={`font-bold ${tx.delta > 0 ? 'text-primary-500' : 'text-red-500'}`}>
                    {tx.delta > 0 ? '+' : ''}{tx.delta} ЯД
                  </p>
                  <p className="text-xs text-gray-400">Баланс: {tx.balance} ЯД</p>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  )
}
