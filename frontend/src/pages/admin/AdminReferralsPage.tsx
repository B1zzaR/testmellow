import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import { Card } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { PageSpinner } from '@/components/ui/Spinner'
import { formatDateTime, formatYAD, formatRubles } from '@/utils/formatters'

export function AdminReferralsPage() {
  const [loginFilter, setLoginFilter] = useState('')
  const [applied, setApplied] = useState('')

  const { data: refsData, isLoading: refsLoading } = useQuery({
    queryKey: ['admin-referrals', applied],
    queryFn: () => adminApi.listAllReferrals(applied ? { login: applied } : undefined),
  })

  const { data: revenueData } = useQuery({
    queryKey: ['admin-revenue', 30],
    queryFn: () => adminApi.getRevenueAnalytics(30),
  })

  if (refsLoading) return <PageSpinner />

  const referrals = refsData?.referrals ?? []
  const topReferrers = revenueData?.top_referrers ?? []

  return (
    <div className="space-y-5">
      <h1 className="text-xl font-bold text-slate-100">Рефералы</h1>

      {/* Top referrers */}
      {topReferrers.length > 0 && (
        <Card title="Топ рефереров">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-surface-700 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                  <th className="pb-3 pr-4">Пользователь</th>
                  <th className="pb-3 pr-4">Рефералов</th>
                  <th className="pb-3">Заработано</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-surface-700">
                {topReferrers.map((tr) => (
                  <tr key={tr.user_id} className="text-slate-300 hover:bg-surface-700/30">
                    <td className="py-3 pr-4">
                      <Link
                        to={`/admin/users/${tr.user_id}`}
                        className="font-medium text-yellow-400 hover:underline"
                      >
                        {tr.username ?? tr.email ?? tr.user_id.slice(0, 8) + '…'}
                      </Link>
                    </td>
                    <td className="py-3 pr-4 font-semibold">{tr.referral_count}</td>
                    <td className="py-3 font-semibold text-yellow-400">
                      {formatYAD(tr.total_reward_yad)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* Filter */}
      <Card>
        <div className="flex flex-wrap gap-3 items-end">
          <div className="flex-1 min-w-48">
            <Input
              label="Логин реферера"
              placeholder="username"
              value={loginFilter}
              onChange={(e) => setLoginFilter(e.target.value)}
            />
          </div>
          <Button onClick={() => setApplied(loginFilter.trim())}>Найти</Button>
          <Button
            variant="secondary"
            onClick={() => { setLoginFilter(''); setApplied('') }}
          >
            Сбросить
          </Button>
        </div>
      </Card>

      {/* Referrals table */}
      <Card>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-surface-700 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                <th className="pb-3 pr-4">Реферер</th>
                <th className="pb-3 pr-4">Реферал</th>
                <th className="pb-3 pr-4">LTV реферала</th>
                <th className="pb-3 pr-4">Вознаграждение</th>
                <th className="pb-3">Дата</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-700">
              {referrals.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-8 text-center text-slate-500">
                    Рефералов не найдено
                  </td>
                </tr>
              )}
              {referrals.map((r) => (
                <tr key={r.id} className="text-slate-300 hover:bg-surface-700/30">
                  <td className="py-3 pr-4">
                    <Link
                      to={`/admin/users/${r.referrer_id}`}
                      className="font-mono text-xs text-yellow-400 hover:underline"
                    >
                      {r.referrer_id.slice(0, 8)}…
                    </Link>
                  </td>
                  <td className="py-3 pr-4">
                    <Link
                      to={`/admin/users/${r.referee_id}`}
                      className="font-mono text-xs text-slate-400 hover:text-slate-200 hover:underline"
                    >
                      {r.referee_id.slice(0, 8)}…
                    </Link>
                  </td>
                  <td className="py-3 pr-4">{formatRubles(r.total_paid_ltv)}</td>
                  <td className="py-3 pr-4 font-semibold text-yellow-400">
                    {formatYAD(r.total_reward)}
                  </td>
                  <td className="py-3 text-slate-400">{formatDateTime(r.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-3 text-xs text-slate-500">Всего: {referrals.length}</p>
      </Card>
    </div>
  )
}
