import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { Modal } from '@/components/ui/Modal'
import { PageSpinner } from '@/components/ui/Spinner'
import { subscriptionStatusBadge } from '@/components/ui/Badge'
import { formatDateTime, formatRubles } from '@/utils/formatters'
import type { SubscriptionStatus } from '@/api/types'

const STATUS_OPTIONS = [
  { value: '', label: 'Все статусы' },
  { value: 'active', label: 'Активна' },
  { value: 'trial', label: 'Пробная' },
  { value: 'expired', label: 'Истекла' },
  { value: 'canceled', label: 'Отменена' },
]

export function AdminSubscriptionsPage() {
  const qc = useQueryClient()
  const [statusFilter, setStatusFilter] = useState('')
  const [flash, setFlash] = useState<{ type: 'success' | 'error'; msg: string } | null>(null)
  const [page, setPage] = useState(1)
  const limit = 50

  // Assign modal
  const [assignOpen, setAssignOpen] = useState(false)
  const [assignLogin, setAssignLogin] = useState('')
  const [assignPlan, setAssignPlan] = useState('1month')

  // Extend modal
  const [extendId, setExtendId] = useState<string | null>(null)
  const [extendDays, setExtendDays] = useState('30')

  // Set-status modal
  const [setStatusId, setSetStatusId] = useState<string | null>(null)
  const [newStatus, setNewStatus] = useState<SubscriptionStatus>('active')

  const params: Record<string, string | number> = { limit, offset: (page - 1) * limit }
  if (statusFilter) params.status = statusFilter

  const { data, isLoading } = useQuery({
    queryKey: ['admin-subscriptions', statusFilter, page],
    queryFn: () => adminApi.listSubscriptions(params),
  })

  const invalidate = () => qc.invalidateQueries({ queryKey: ['admin-subscriptions'] })

  const assignMutation = useMutation({
    mutationFn: ({ login, plan }: { login: string; plan: string }) =>
      adminApi.assignSubscription({ login, plan }),
    onSuccess: (res) => {
      setFlash({ type: 'success', msg: `${res.message} — ${res.login}, истекает ${new Date(res.expires_at).toLocaleString('ru-RU')}` })
      setAssignOpen(false)
      setAssignLogin('')
      setAssignPlan('1month')
      invalidate()
    },
    onError: (e: Error) => setFlash({ type: 'error', msg: e.message }),
  })

  const extendMutation = useMutation({
    mutationFn: ({ id, days }: { id: string; days: number }) =>
      adminApi.extendSubscription(id, { days }),
    onSuccess: (res) => {
      setFlash({ type: 'success', msg: res.message })
      setExtendId(null)
      invalidate()
    },
    onError: (e: Error) => setFlash({ type: 'error', msg: e.message }),
  })

  const setStatusMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: SubscriptionStatus }) =>
      adminApi.setSubscriptionStatus(id, { status }),
    onSuccess: (res) => {
      setFlash({ type: 'success', msg: res.message })
      setSetStatusId(null)
      invalidate()
    },
    onError: (e: Error) => setFlash({ type: 'error', msg: e.message }),
  })

  if (isLoading) return <PageSpinner />

  const subs = data?.subscriptions ?? []

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-slate-100">Подписки</h1>
        <Button onClick={() => { setAssignLogin(''); setAssignPlan('1month'); setAssignOpen(true) }}>
          + Назначить подписку
        </Button>
      </div>

      {flash && <Alert variant={flash.type} message={flash.msg} />}

      {/* Filter */}
      <Card>
        <div className="flex flex-wrap gap-3 items-end">
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-400">Статус</label>
            <select
              value={statusFilter}
              onChange={(e) => { setStatusFilter(e.target.value); setPage(1) }}
              className="rounded-lg border border-surface-600 bg-surface-700 px-3 py-2 text-sm text-slate-200 focus:outline-none focus:ring-2 focus:ring-yellow-500"
            >
              {STATUS_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </div>
          <Button variant="secondary" onClick={() => { setStatusFilter(''); setPage(1) }}>
            Сбросить
          </Button>
        </div>
      </Card>

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-surface-700 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                <th className="pb-3 pr-4">Пользователь</th>
                <th className="pb-3 pr-4">Тариф</th>
                <th className="pb-3 pr-4">Статус</th>
                <th className="pb-3 pr-4">Истекает</th>
                <th className="pb-3 pr-4">Оплачено</th>
                <th className="pb-3">Действия</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-surface-700">
              {subs.length === 0 && (
                <tr>
                  <td colSpan={6} className="py-8 text-center text-slate-500">
                    Подписок не найдено
                  </td>
                </tr>
              )}
              {subs.map((s) => (
                <tr key={s.id} className="text-slate-300 hover:bg-surface-700/30">
                  <td className="py-3 pr-4 font-mono text-xs text-slate-400">{s.user_id.slice(0, 8)}…</td>
                  <td className="py-3 pr-4 capitalize">{s.plan}</td>
                  <td className="py-3 pr-4">{subscriptionStatusBadge(s.status)}</td>
                  <td className="py-3 pr-4 text-slate-400">{formatDateTime(s.expires_at)}</td>
                  <td className="py-3 pr-4 font-semibold">{formatRubles(s.paid_kopecks)}</td>
                  <td className="py-3">
                    <div className="flex gap-2">
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => { setExtendId(s.id); setExtendDays('30') }}
                      >
                        Продлить
                      </Button>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => { setSetStatusId(s.id); setNewStatus(s.status) }}
                      >
                        Статус
                      </Button>
                    </div>
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

      {/* Assign subscription modal */}
      <Modal
        open={assignOpen}
        onClose={() => setAssignOpen(false)}
        title="Назначить подписку"
        footer={
          <>
            <Button variant="secondary" onClick={() => setAssignOpen(false)}>Отмена</Button>
            <Button
              loading={assignMutation.isPending}
              disabled={!assignLogin.trim()}
              onClick={() => {
                if (assignLogin.trim()) assignMutation.mutate({ login: assignLogin.trim(), plan: assignPlan })
              }}
            >
              Назначить
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Логин пользователя"
            placeholder="username"
            value={assignLogin}
            onChange={(e) => setAssignLogin(e.target.value)}
          />
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-400">Тариф</label>
            <select
              value={assignPlan}
              onChange={(e) => setAssignPlan(e.target.value)}
              className="w-full rounded-lg border border-surface-600 bg-surface-700 px-3 py-2 text-sm text-slate-200 focus:outline-none focus:ring-2 focus:ring-yellow-500"
            >
              <option value="1week">1 неделя (7 дней)</option>
              <option value="1month">1 месяц (30 дней)</option>
              <option value="3months">3 месяца (90 дней)</option>
            </select>
          </div>
          <p className="text-xs text-slate-500">
            Если у пользователя уже есть активная подписка — дни будут добавлены сверху.
          </p>
        </div>
      </Modal>

      {/* Extend modal */}
      <Modal
        open={Boolean(extendId)}
        onClose={() => setExtendId(null)}
        title="Продлить подписку"
        footer={
          <>
            <Button variant="secondary" onClick={() => setExtendId(null)}>Отмена</Button>
            <Button
              loading={extendMutation.isPending}
              onClick={() => {
                const d = parseInt(extendDays, 10)
                if (extendId && d > 0) extendMutation.mutate({ id: extendId, days: d })
              }}
            >
              Продлить
            </Button>
          </>
        }
      >
        <Input
          label="Количество дней"
          type="number"
          min={1}
          max={3650}
          value={extendDays}
          onChange={(e) => setExtendDays(e.target.value)}
        />
      </Modal>

      {/* Set-status modal */}
      <Modal
        open={Boolean(setStatusId)}
        onClose={() => setSetStatusId(null)}
        title="Изменить статус подписки"
        footer={
          <>
            <Button variant="secondary" onClick={() => setSetStatusId(null)}>Отмена</Button>
            <Button
              loading={setStatusMutation.isPending}
              onClick={() => {
                if (setStatusId) setStatusMutation.mutate({ id: setStatusId, status: newStatus })
              }}
            >
              Сохранить
            </Button>
          </>
        }
      >
        <div>
          <label className="mb-1 block text-xs font-medium text-slate-400">Новый статус</label>
          <select
            value={newStatus}
            onChange={(e) => setNewStatus(e.target.value as SubscriptionStatus)}
            className="w-full rounded-lg border border-surface-600 bg-surface-700 px-3 py-2 text-sm text-slate-200 focus:outline-none focus:ring-2 focus:ring-yellow-500"
          >
            <option value="active">Активна</option>
            <option value="expired">Истекла</option>
            <option value="canceled">Отменена</option>
          </select>
        </div>
      </Modal>
    </div>
  )
}
