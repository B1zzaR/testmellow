import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { Modal } from '@/components/ui/Modal'
import { Badge } from '@/components/ui/Badge'
import { PageSpinner } from '@/components/ui/Spinner'
import { formatDateTime, formatYAD } from '@/utils/formatters'
import type { YADTxType } from '@/api/types'

const TX_TYPE_OPTIONS = [
  { value: '', label: 'Все типы' },
  { value: 'referral_reward', label: 'Реферальная награда' },
  { value: 'bonus', label: 'Бонус' },
  { value: 'spent', label: 'Потрачено' },
  { value: 'promo', label: 'Промокод' },
  { value: 'trial', label: 'Пробный' },
]

function txTypeBadge(type: YADTxType) {
  const map: Record<string, 'green' | 'yellow' | 'red' | 'blue' | 'gray' | 'purple'> = {
    referral_reward: 'green',
    bonus: 'yellow',
    spent: 'red',
    promo: 'blue',
    trial: 'purple',
  }
  const labels: Record<string, string> = {
    referral_reward: 'Реферал',
    bonus: 'Бонус',
    spent: 'Списание',
    promo: 'Промокод',
    trial: 'Пробный',
  }
  return <Badge label={labels[type] ?? type} variant={map[type] ?? 'gray'} />
}

export function AdminYADPage() {
  const qc = useQueryClient()
  const [loginFilter, setLoginFilter] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [appliedLogin, setAppliedLogin] = useState('')
  const [flash, setFlash] = useState<{ type: 'success' | 'error'; msg: string } | null>(null)

  // Adjust modal
  const [adjustOpen, setAdjustOpen] = useState(false)
  const [adjUserId, setAdjUserId] = useState('')
  const [adjDelta, setAdjDelta] = useState('')
  const [adjNote, setAdjNote] = useState('')

  const params: Record<string, string> = {}
  if (appliedLogin) params.login = appliedLogin
  if (typeFilter) params.type = typeFilter

  const { data, isLoading } = useQuery({
    queryKey: ['admin-yad', appliedLogin, typeFilter],
    queryFn: () => adminApi.listAllYADTransactions(params),
  })

  const adjustMutation = useMutation({
    mutationFn: () =>
      adminApi.adminAdjustYAD({ user_id: adjUserId, delta: Number(adjDelta), note: adjNote }),
    onSuccess: () => {
      setFlash({ type: 'success', msg: 'Баланс скорректирован' })
      setAdjustOpen(false)
      setAdjUserId('')
      setAdjDelta('')
      setAdjNote('')
      qc.invalidateQueries({ queryKey: ['admin-yad'] })
    },
    onError: (e: Error) => setFlash({ type: 'error', msg: e.message }),
  })

  if (isLoading) return <PageSpinner />

  const txs = data?.transactions ?? []

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-slate-100">ЯД-экономика</h1>
        <Button onClick={() => setAdjustOpen(true)}>Скорректировать баланс</Button>
      </div>

      {flash && <Alert variant={flash.type} message={flash.msg} />}

      {/* Filters */}
      <Card>
        <div className="flex flex-wrap gap-3 items-end">
          <div className="flex-1 min-w-48">
            <Input
              label="Логин пользователя"
              placeholder="username"
              value={loginFilter}
              onChange={(e) => setLoginFilter(e.target.value)}
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-400">Тип транзакции</label>
            <select
              value={typeFilter}
              onChange={(e) => setTypeFilter(e.target.value)}
              className="rounded-lg border border-surface-600 bg-surface-700 px-3 py-2 text-sm text-slate-200 focus:outline-none focus:ring-2 focus:ring-yellow-500"
            >
              {TX_TYPE_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </div>
          <Button onClick={() => setAppliedLogin(loginFilter.trim())}>Найти</Button>
          <Button
            variant="secondary"
            onClick={() => { setLoginFilter(''); setAppliedLogin(''); setTypeFilter('') }}
          >
            Сбросить
          </Button>
        </div>
      </Card>

      {/* Transactions table */}
      <Card>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-surface-700 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                <th className="pb-3 pr-4">Пользователь</th>
                <th className="pb-3 pr-4">Изменение</th>
                <th className="pb-3 pr-4">Баланс после</th>
                <th className="pb-3 pr-4">Тип</th>
                <th className="pb-3 pr-4">Заметка</th>
                <th className="pb-3">Дата</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-700">
              {txs.length === 0 && (
                <tr>
                  <td colSpan={6} className="py-8 text-center text-slate-500">
                    Транзакций не найдено
                  </td>
                </tr>
              )}
              {txs.map((tx) => (
                <tr key={tx.id} className="text-slate-300 hover:bg-surface-700/30">
                  <td className="py-3 pr-4">
                    <Link
                      to={`/admin/users/${tx.user_id}`}
                      className="font-mono text-xs text-yellow-400 hover:underline"
                    >
                      {tx.user_id.slice(0, 8)}…
                    </Link>
                  </td>
                  <td className="py-3 pr-4">
                    <span
                      className={`font-semibold ${tx.delta >= 0 ? 'text-green-400' : 'text-red-400'}`}
                    >
                      {tx.delta >= 0 ? '+' : ''}{tx.delta} ЯД
                    </span>
                  </td>
                  <td className="py-3 pr-4 text-slate-400">{formatYAD(tx.balance)}</td>
                  <td className="py-3 pr-4">{txTypeBadge(tx.tx_type)}</td>
                  <td className="py-3 pr-4 max-w-xs truncate text-slate-400" title={tx.note}>
                    {tx.note || '—'}
                  </td>
                  <td className="py-3 text-slate-400">{formatDateTime(tx.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="mt-3 text-xs text-slate-500">Всего: {txs.length}</p>
      </Card>

      {/* Adjust modal */}
      <Modal
        open={adjustOpen}
        onClose={() => setAdjustOpen(false)}
        title="Корректировка баланса ЯД"
        footer={
          <>
            <Button variant="secondary" onClick={() => setAdjustOpen(false)}>Отмена</Button>
            <Button
              loading={adjustMutation.isPending}
              onClick={() => {
                if (adjUserId && adjDelta && adjNote) adjustMutation.mutate()
              }}
            >
              Применить
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="ID пользователя"
            placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
            value={adjUserId}
            onChange={(e) => setAdjUserId(e.target.value)}
          />
          <Input
            label="Изменение ЯД (отрицательное для списания)"
            type="number"
            value={adjDelta}
            onChange={(e) => setAdjDelta(e.target.value)}
          />
          <Input
            label="Причина / заметка"
            placeholder="Например: ручная корректировка"
            value={adjNote}
            onChange={(e) => setAdjNote(e.target.value)}
          />
        </div>
      </Modal>
    </div>
  )
}
