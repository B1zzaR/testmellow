import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { PageSpinner } from '@/components/ui/Spinner'
import { paymentStatusBadge } from '@/components/ui/Badge'
import { formatDateTime, formatRubles } from '@/utils/formatters'
import type { PaymentStatus } from '@/api/types'

const STATUS_OPTIONS: { value: string; label: string }[] = [
  { value: '', label: 'Все статусы' },
  { value: 'PENDING', label: 'Ожидание' },
  { value: 'CONFIRMED', label: 'Подтверждён' },
  { value: 'CANCELED', label: 'Отменён' },
  { value: 'CHARGEBACKED', label: 'Возврат' },
  { value: 'EXPIRED', label: 'Истёк' },
]

export function AdminPaymentsPage() {
  const qc = useQueryClient()
  const navigate = useNavigate()
  const [status, setStatus] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [flash, setFlash] = useState<{ type: 'success' | 'error'; msg: string } | null>(null)
  const [page, setPage] = useState(1)
  const limit = 50

  const params: Record<string, string | number> = { limit, offset: (page - 1) * limit }
  if (status) params.status = status
  if (from) params.from = new Date(from).toISOString()
  if (to) params.to = new Date(to).toISOString()

  const { data, isLoading } = useQuery({
    queryKey: ['admin-payments', status, from, to, page],
    queryFn: () => adminApi.listPayments(params),
  })

  const checkMutation = useMutation({
    mutationFn: (id: string) => adminApi.checkPaymentStatus(id),
    onSuccess: (res) => {
      setFlash({ type: 'success', msg: `Статус Platega: ${res.platega_status} / БД: ${res.db_status}` })
      qc.invalidateQueries({ queryKey: ['admin-payments'] })
    },
    onError: (e: Error) => setFlash({ type: 'error', msg: e.message }),
  })

  if (isLoading) return <PageSpinner />

  const payments = data?.payments ?? []

  return (
    <div className="space-y-5">
      <h1 className="text-xl font-bold text-slate-100">Платежи</h1>

      {flash && (
        <Alert
          variant={flash.type}
          message={flash.msg}
        />
      )}

      {/* Filters */}
      <Card>
        <div className="flex flex-wrap gap-3 items-end">
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-400">Статус</label>
            <select
              value={status}
              onChange={(e) => { setStatus(e.target.value); setPage(1) }}
              className="rounded-lg border border-surface-600 bg-surface-700 px-3 py-2 text-sm text-slate-200 focus:outline-none focus:ring-2 focus:ring-yellow-500"
            >
              {STATUS_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </div>
          <div className="w-44">
            <Input
              label="От (дата)"
              type="date"
              value={from}
              onChange={(e) => { setFrom(e.target.value); setPage(1) }}
            />
          </div>
          <div className="w-44">
            <Input
              label="До (дата)"
              type="date"
              value={to}
              onChange={(e) => { setTo(e.target.value); setPage(1) }}
            />
          </div>
          <Button
            variant="secondary"
            onClick={() => { setStatus(''); setFrom(''); setTo(''); setPage(1) }}
          >
            Сбросить
          </Button>
        </div>
      </Card>

      {/* Table */}
      <Card>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-surface-700 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                <th className="pb-3 pr-4">ID</th>
                <th className="pb-3 pr-4">Пользователь</th>
                <th className="pb-3 pr-4">Тариф</th>
                <th className="pb-3 pr-4">Сумма</th>
                <th className="pb-3 pr-4">Статус</th>
                <th className="pb-3 pr-4">Создан</th>
                <th className="pb-3">Действия</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-700">
              {payments.length === 0 && (
                <tr>
                  <td colSpan={7} className="py-8 text-center text-slate-500">
                    Платежей не найдено
                  </td>
                </tr>
              )}
              {payments.map((p) => (
                <tr key={p.id} className="text-slate-300 transition-colors hover:bg-surface-700/30">
                  <td className="py-3 pr-4">
                    <span className="font-mono text-xs text-slate-500">{p.id.slice(0, 8)}…</span>
                  </td>
                  <td className="py-3 pr-4">
                    <button
                      className="font-mono text-xs text-yellow-400 hover:underline"
                      onClick={() => navigate(`/admin/users/${p.user_id}`)}
                    >
                      {p.username ?? `${p.user_id.slice(0, 8)}…`}
                    </button>
                  </td>
                  <td className="py-3 pr-4 capitalize">{p.plan}</td>
                  <td className="py-3 pr-4 font-semibold">{formatRubles(p.amount_kopecks)}</td>
                  <td className="py-3 pr-4">{paymentStatusBadge(p.status as PaymentStatus)}</td>
                  <td className="py-3 pr-4 text-slate-400">{formatDateTime(p.created_at)}</td>
                  <td className="py-3">
                    {p.status === 'PENDING' && (
                      <Button
                        variant="secondary"
                        size="sm"
                        loading={checkMutation.isPending && checkMutation.variables === p.id}
                        onClick={() => checkMutation.mutate(p.id)}
                      >
                        Проверить
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-3 text-xs text-slate-500">Всего: {data?.total ?? 0}</p>
        {(data?.total ?? 0) > limit && (
          <div className="mt-3 flex items-center gap-3">
            <Button variant="secondary" size="sm" disabled={page === 1} onClick={() => setPage((p) => p - 1)}>← Назад</Button>
            <span className="text-xs text-slate-400">Страница {page} из {Math.ceil((data?.total ?? 0) / limit)}</span>
            <Button variant="secondary" size="sm" disabled={page * limit >= (data?.total ?? 0)} onClick={() => setPage((p) => p + 1)}>Вперёд →</Button>
          </div>
        )}
      </Card>
    </div>
  )
}
