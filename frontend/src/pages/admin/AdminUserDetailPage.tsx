import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { Badge } from '@/components/ui/Badge'
import { Modal } from '@/components/ui/Modal'
import { PageSpinner } from '@/components/ui/Spinner'
import { subscriptionStatusBadge, paymentStatusBadge } from '@/components/ui/Badge'
import { formatDateTime, formatRubles, formatYAD, planLabel } from '@/utils/formatters'
import type { YADTxType, SubscriptionStatus } from '@/api/types'

type Tab = 'info' | 'subscriptions' | 'payments' | 'yad'

const TAB_LABELS: Record<Tab, string> = {
  info: 'Профиль',
  subscriptions: 'Подписки',
  payments: 'Платежи',
  yad: 'ЯД история',
}

export function AdminUserDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [activeTab, setActiveTab] = useState<Tab>('info')
  const [riskModalOpen, setRiskModalOpen] = useState(false)
  const [riskScore, setRiskScore] = useState('')
  const [adjustYADOpen, setAdjustYADOpen] = useState(false)
  const [adjDelta, setAdjDelta] = useState('')
  const [adjNote, setAdjNote] = useState('')
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')

  // Subscriptions tab actions
  const [extendSubId, setExtendSubId] = useState<string | null>(null)
  const [extendSubDays, setExtendSubDays] = useState('30')
  const [setSubStatusId, setSetSubStatusId] = useState<string | null>(null)
  const [newSubStatus, setNewSubStatus] = useState<SubscriptionStatus>('active')

  const { data: user, isLoading } = useQuery({
    queryKey: ['admin-user', id],
    queryFn: () => adminApi.getUser(id!),
    enabled: Boolean(id),
  })

  const { data: subsData } = useQuery({
    queryKey: ['admin-user-subs', id],
    queryFn: () => adminApi.getUserSubscriptions(id!),
    enabled: activeTab === 'subscriptions' && Boolean(id),
  })

  const { data: paymentsData } = useQuery({
    queryKey: ['admin-user-payments', id],
    queryFn: () => adminApi.getUserPayments(id!),
    enabled: activeTab === 'payments' && Boolean(id),
  })

  const { data: yadData } = useQuery({
    queryKey: ['admin-user-yad', id],
    queryFn: () => adminApi.getUserYAD(id!),
    enabled: activeTab === 'yad' && Boolean(id),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-user', id] })
    queryClient.invalidateQueries({ queryKey: ['admin-users'] })
  }

  const banMutation = useMutation({
    mutationFn: () => adminApi.banUser(id!),
    onSuccess: () => { setSuccessMsg('Заблокирован'); invalidate() },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const unbanMutation = useMutation({
    mutationFn: () => adminApi.unbanUser(id!),
    onSuccess: () => { setSuccessMsg('Разблокирован'); invalidate() },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const riskMutation = useMutation({
    mutationFn: (score: number) => adminApi.setRiskScore(id!, { score }),
    onSuccess: () => {
      setSuccessMsg('Уровень риска обновлён')
      setRiskModalOpen(false)
      invalidate()
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const adjustYADMutation = useMutation({
    mutationFn: () =>
      adminApi.adjustUserYAD(id!, { delta: Number(adjDelta), note: adjNote }),
    onSuccess: () => {
      setSuccessMsg('Баланс ЯД скорректирован')
      setAdjustYADOpen(false)
      setAdjDelta('')
      setAdjNote('')
      queryClient.invalidateQueries({ queryKey: ['admin-user', id] })
      queryClient.invalidateQueries({ queryKey: ['admin-user-yad', id] })
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const extendSubMutation = useMutation({
    mutationFn: ({ subId, days }: { subId: string; days: number }) =>
      adminApi.extendSubscription(subId, { days }),
    onSuccess: (res) => {
      setSuccessMsg(res.message)
      setExtendSubId(null)
      queryClient.invalidateQueries({ queryKey: ['admin-user-subs', id] })
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const setSubStatusMutation = useMutation({
    mutationFn: ({ subId, status }: { subId: string; status: SubscriptionStatus }) =>
      adminApi.setSubscriptionStatus(subId, { status }),
    onSuccess: (res) => {
      setSuccessMsg(res.message)
      setSetSubStatusId(null)
      queryClient.invalidateQueries({ queryKey: ['admin-user-subs', id] })
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const checkPaymentMutation = useMutation({
    mutationFn: (paymentId: string) => adminApi.checkPaymentStatus(paymentId),
    onSuccess: (res) => {
      setSuccessMsg(`Platega: ${res.platega_status} / БД: ${res.db_status}`)
      queryClient.invalidateQueries({ queryKey: ['admin-user-payments', id] })
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  if (isLoading) return <PageSpinner />
  if (!user) return <Alert variant="error" message="Пользователь не найден" />

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        <button
          onClick={() => navigate('/admin/users')}
          className="rounded-lg p-2 text-slate-500 hover:bg-surface-700 transition-colors"
        >
          ←
        </button>
        <h1 className="text-xl font-bold text-slate-100">
          {user.email ?? user.username ?? user.id}
        </h1>
        <div className="flex gap-1">
          {user.is_admin && <Badge label="Админ" variant="purple" />}
          {user.is_banned && <Badge label="Заблокирован" variant="red" />}
          {!user.is_banned && !user.is_admin && <Badge label="Активен" variant="green" />}
        </div>
      </div>

      {successMsg && <Alert variant="success" message={successMsg} />}
      {errorMsg && <Alert variant="error" message={errorMsg} />}

      {/* Tabs */}
      <div className="flex gap-1 border-b border-surface-700">
        {(Object.keys(TAB_LABELS) as Tab[]).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={[
              'px-4 py-2.5 text-sm font-medium transition-colors border-b-2 -mb-px',
              activeTab === tab
                ? 'border-yellow-500 text-yellow-500'
                : 'border-transparent text-slate-400 hover:text-slate-200',
            ].join(' ')}
          >
            {TAB_LABELS[tab]}
          </button>
        ))}
      </div>

      {/* ─── Profile tab ────────────────────────────────────────── */}
      {activeTab === 'info' && (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            <Card title="Данные аккаунта">
              <dl className="space-y-3 text-sm">
                <div>
                  <dt className="text-slate-500">ID пользователя</dt>
                  <dd className="mt-0.5 font-mono text-xs text-slate-400">{user.id}</dd>
                </div>
                <div>
                  <dt className="text-slate-500">Логин</dt>
                  <dd className="mt-0.5 font-medium">{user.username ?? '—'}</dd>
                </div>
                <div>
                  <dt className="text-slate-500">Реферальный код</dt>
                  <dd className="mt-0.5 font-mono">{user.referral_code}</dd>
                </div>
                <div>
                  <dt className="text-slate-500">Зарегистрирован</dt>
                  <dd className="mt-0.5">{formatDateTime(user.created_at)}</dd>
                </div>
                <div>
                  <dt className="text-slate-500">Пробный использован</dt>
                  <dd className="mt-0.5">{user.trial_used ? 'Да' : 'Нет'}</dd>
                </div>
              </dl>
            </Card>

            <Card title="Финансы">
              <dl className="space-y-3 text-sm">
                <div>
                  <dt className="text-slate-500">Баланс ЯД</dt>
                  <dd className="mt-0.5 font-semibold text-lg">{formatYAD(user.yad_balance)}</dd>
                </div>
                <div>
                  <dt className="text-slate-500">LTV</dt>
                  <dd className="mt-0.5 font-semibold">{formatRubles(user.ltv_kopecks)}</dd>
                </div>
                <div>
                  <dt className="text-slate-500">Уровень риска</dt>
                  <dd
                    className={`mt-0.5 font-bold text-lg ${
                      user.risk_score >= 70
                        ? 'text-red-400'
                        : user.risk_score >= 40
                        ? 'text-yellow-400'
                        : 'text-green-400'
                    }`}
                  >
                    {user.risk_score} / 100
                  </dd>
                </div>
              </dl>
            </Card>
          </div>

          <Card title="Действия">
            <div className="flex flex-wrap gap-3">
              {user.is_banned ? (
                <Button
                  variant="secondary"
                  loading={unbanMutation.isPending}
                  onClick={() => unbanMutation.mutate()}
                >
                  Разблокировать
                </Button>
              ) : (
                <Button
                  variant="danger"
                  loading={banMutation.isPending}
                  onClick={() => banMutation.mutate()}
                >
                  Заблокировать
                </Button>
              )}
              <Button
                variant="secondary"
                onClick={() => {
                  setRiskScore(String(user.risk_score))
                  setRiskModalOpen(true)
                }}
              >
                Установить риск
              </Button>
              <Button
                variant="secondary"
                onClick={() => { setAdjDelta(''); setAdjNote(''); setAdjustYADOpen(true) }}
              >
                Скорректировать ЯД
              </Button>
            </div>
          </Card>
        </>
      )}

      {/* ─── Subscriptions tab ──────────────────────────────────── */}
      {activeTab === 'subscriptions' && (
        <Card title="Подписки пользователя">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-surface-700 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                  <th className="pb-3 pr-4">Тариф</th>
                  <th className="pb-3 pr-4">Статус</th>
                  <th className="pb-3 pr-4">Истекает</th>
                  <th className="pb-3 pr-4">Оплачено</th>
                  <th className="pb-3">Действия</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-surface-700">
                {(subsData?.subscriptions ?? []).length === 0 && (
                  <tr>
                    <td colSpan={5} className="py-8 text-center text-slate-500">
                      Нет подписок
                    </td>
                  </tr>
                )}
                {(subsData?.subscriptions ?? []).map((s) => (
                  <tr key={s.id} className="text-slate-300 hover:bg-surface-700/30">
                    <td className="py-3 pr-4">{planLabel(s.plan)}</td>
                    <td className="py-3 pr-4">{subscriptionStatusBadge(s.status)}</td>
                    <td className="py-3 pr-4 text-slate-400">{formatDateTime(s.expires_at)}</td>
                    <td className="py-3 pr-4 font-semibold">{formatRubles(s.paid_kopecks)}</td>
                    <td className="py-3">
                      <div className="flex gap-2">
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => { setExtendSubId(s.id); setExtendSubDays('30') }}
                        >
                          Продлить
                        </Button>
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => { setSetSubStatusId(s.id); setNewSubStatus(s.status) }}
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
        </Card>
      )}

      {/* ─── Payments tab ───────────────────────────────────────── */}
      {activeTab === 'payments' && (
        <Card title="Платежи пользователя">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-surface-700 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                  <th className="pb-3 pr-4">Тариф</th>
                  <th className="pb-3 pr-4">Сумма</th>
                  <th className="pb-3 pr-4">Статус</th>
                  <th className="pb-3 pr-4">Дата</th>
                  <th className="pb-3">Действия</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-surface-700">
                {(paymentsData?.payments ?? []).length === 0 && (
                  <tr>
                    <td colSpan={5} className="py-8 text-center text-slate-500">
                      Нет платежей
                    </td>
                  </tr>
                )}
                {(paymentsData?.payments ?? []).map((p) => (
                  <tr key={p.id} className="text-slate-300 hover:bg-surface-700/30">
                    <td className="py-3 pr-4">{planLabel(p.plan)}</td>
                    <td className="py-3 pr-4 font-semibold">{formatRubles(p.amount_kopecks)}</td>
                    <td className="py-3 pr-4">{paymentStatusBadge(p.status)}</td>
                    <td className="py-3 pr-4 text-slate-400">{formatDateTime(p.created_at)}</td>
                    <td className="py-3">
                      {p.status === 'PENDING' && (
                        <Button
                          variant="secondary"
                          size="sm"
                          loading={checkPaymentMutation.isPending && checkPaymentMutation.variables === p.id}
                          onClick={() => checkPaymentMutation.mutate(p.id)}
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
        </Card>
      )}

      {/* ─── YAD history tab ────────────────────────────────────── */}
      {activeTab === 'yad' && (
        <Card title="История ЯД">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-surface-700 text-left text-xs font-semibold uppercase tracking-wider text-slate-500">
                  <th className="pb-3 pr-4">Изменение</th>
                  <th className="pb-3 pr-4">Баланс</th>
                  <th className="pb-3 pr-4">Тип</th>
                  <th className="pb-3 pr-4">Заметка</th>
                  <th className="pb-3">Дата</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-surface-700">
                {(yadData?.transactions ?? []).length === 0 && (
                  <tr>
                    <td colSpan={5} className="py-8 text-center text-slate-500">
                      Нет транзакций
                    </td>
                  </tr>
                )}
                {(yadData?.transactions ?? []).map((tx) => (
                  <tr key={tx.id} className="text-slate-300 hover:bg-surface-700/30">
                    <td className="py-3 pr-4">
                      <span
                        className={`font-semibold ${tx.delta >= 0 ? 'text-green-400' : 'text-red-400'}`}
                      >
                        {tx.delta >= 0 ? '+' : ''}{tx.delta} ЯД
                      </span>
                    </td>
                    <td className="py-3 pr-4 text-slate-400">{formatYAD(tx.balance)}</td>
                    <td className="py-3 pr-4">
                      <Badge label={tx.tx_type as YADTxType} variant="gray" />
                    </td>
                    <td className="py-3 pr-4 max-w-xs truncate text-slate-400">{tx.note || '—'}</td>
                    <td className="py-3 text-slate-400">{formatDateTime(tx.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* Risk modal */}
      <Modal
        open={riskModalOpen}
        onClose={() => setRiskModalOpen(false)}
        title="Обновить уровень риска"
        footer={
          <>
            <Button variant="secondary" onClick={() => setRiskModalOpen(false)}>
              Отмена
            </Button>
            <Button
              loading={riskMutation.isPending}
              onClick={() => {
                const n = parseInt(riskScore, 10)
                if (!isNaN(n) && n >= 0 && n <= 100) riskMutation.mutate(n)
              }}
            >
              Сохранить
            </Button>
          </>
        }
      >
        <Input
          label="Уровень риска (0–100)"
          type="number"
          min={0}
          max={100}
          value={riskScore}
          onChange={(e) => setRiskScore(e.target.value)}
        />
        <p className="mt-2 text-xs text-slate-500">
          0–39 = Низкий, 40–69 = Средний, 70–100 = Высокий риск
        </p>
      </Modal>

      {/* Adjust YAD modal */}
      <Modal
        open={adjustYADOpen}
        onClose={() => setAdjustYADOpen(false)}
        title="Корректировка баланса ЯД"
        footer={
          <>
            <Button variant="secondary" onClick={() => setAdjustYADOpen(false)}>
              Отмена
            </Button>
            <Button
              loading={adjustYADMutation.isPending}
              onClick={() => {
                if (adjDelta && adjNote) adjustYADMutation.mutate()
              }}
            >
              Применить
            </Button>
          </>
        }
      >
        <div className="space-y-4">
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

      {/* Extend subscription modal */}
      <Modal
        open={extendSubId !== null}
        onClose={() => setExtendSubId(null)}
        title="Продлить подписку"
        footer={
          <>
            <Button variant="secondary" onClick={() => setExtendSubId(null)}>
              Отмена
            </Button>
            <Button
              loading={extendSubMutation.isPending}
              onClick={() => {
                if (extendSubId) extendSubMutation.mutate({ subId: extendSubId, days: Number(extendSubDays) })
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
          value={extendSubDays}
          onChange={(e) => setExtendSubDays(e.target.value)}
        />
      </Modal>

      {/* Set subscription status modal */}
      <Modal
        open={setSubStatusId !== null}
        onClose={() => setSetSubStatusId(null)}
        title="Изменить статус подписки"
        footer={
          <>
            <Button variant="secondary" onClick={() => setSetSubStatusId(null)}>
              Отмена
            </Button>
            <Button
              loading={setSubStatusMutation.isPending}
              onClick={() => {
                if (setSubStatusId) setSubStatusMutation.mutate({ subId: setSubStatusId, status: newSubStatus })
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
            value={newSubStatus}
            onChange={(e) => setNewSubStatus(e.target.value as SubscriptionStatus)}
            className="w-full rounded-lg border border-surface-600 bg-surface-700 px-3 py-2 text-sm text-slate-200 focus:outline-none focus:ring-2 focus:ring-yellow-500"
          >
            <option value="active">Активна</option>
            <option value="expired">Истекла</option>
            <option value="canceled">Отменена</option>
            <option value="trial">Пробная</option>
          </select>
        </div>
      </Modal>
    </div>
  )
}

