import { useQuery } from '@tanstack/react-query'
import { balanceApi } from '@/api/profile'
import { Card, StatCard } from '@/components/ui/Card'
import { PageSpinner } from '@/components/ui/Spinner'
import { Alert } from '@/components/ui/Alert'
import { formatDateTime, formatYAD } from '@/utils/formatters'
import type { YADTxType } from '@/api/types'

const txTypeLabel: Record<YADTxType, string> = {
  referral_reward: 'Referral Reward',
  bonus: 'Bonus',
  spent: 'Spent',
  promo: 'Promo Code',
  trial: 'Trial',
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
      <h1 className="text-2xl font-bold text-gray-900">YAD Balance</h1>

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard
          label="Current Balance"
          value={formatYAD(balance?.yad_balance ?? 0)}
          sub={`≈ ${(balance?.yad_ruble_value ?? 0).toFixed(2)} ₽`}
          icon="💎"
        />
        <StatCard
          label="Exchange Rate"
          value="1 YAD = 2.50 ₽"
          sub="Fixed rate"
          icon="💱"
        />
        <StatCard
          label="Value"
          value={`${(balance?.yad_ruble_value ?? 0).toFixed(2)} ₽`}
          sub="Total ruble value"
          icon="💰"
        />
      </div>

      <Card title="Transaction History">
        {histLoading ? (
          <PageSpinner />
        ) : !history?.transactions?.length ? (
          <Alert variant="info" message="No transactions yet" />
        ) : (
          <div className="space-y-1">
            {history.transactions.map((tx) => (
              <div
                key={tx.id}
                className="flex items-center justify-between rounded-lg px-4 py-3 hover:bg-gray-50"
              >
                <div>
                  <p className="text-sm font-medium text-gray-800">
                    {txTypeLabel[tx.tx_type] ?? tx.tx_type}
                  </p>
                  {tx.note && <p className="text-xs text-gray-400">{tx.note}</p>}
                  <p className="text-xs text-gray-400">{formatDateTime(tx.created_at)}</p>
                </div>
                <div className="text-right">
                  <p
                    className={`font-semibold ${
                      tx.delta > 0 ? 'text-green-600' : 'text-red-600'
                    }`}
                  >
                    {tx.delta > 0 ? '+' : ''}{tx.delta} YAD
                  </p>
                  <p className="text-xs text-gray-400">Balance: {tx.balance} YAD</p>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  )
}
