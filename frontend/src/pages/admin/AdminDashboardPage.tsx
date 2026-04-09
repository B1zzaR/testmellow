import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { adminApi } from '@/api/admin'
import { StatCard, Card } from '@/components/ui/Card'
import { Icon } from '@/components/ui/Icons'
import { PageSpinner } from '@/components/ui/Spinner'
import { Alert } from '@/components/ui/Alert'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Modal } from '@/components/ui/Modal'
import { formatRubles, formatYAD } from '@/utils/formatters'

const PLAN_OPTIONS = [
  { value: '1week',   label: '1 неделя'  },
  { value: '1month',  label: '1 месяц'   },
  { value: '3months', label: '3 месяца'  },
]

export function AdminDashboardPage() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['admin-analytics'],
    queryFn: adminApi.getAnalytics,
    refetchInterval: 30_000,
  })

  const { data: settings, isLoading: settingsLoading } = useQuery({
    queryKey: ['admin-settings'],
    queryFn: adminApi.getSettings,
  })

  const [assignLogin, setAssignLogin] = useState('')
  const [assignPlan, setAssignPlan] = useState('1month')
  const [assignMsg, setAssignMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [showConfirmModal, setShowConfirmModal] = useState(false)

  const assignMutation = useMutation({
    mutationFn: () => adminApi.assignSubscription({ login: assignLogin.trim(), plan: assignPlan }),
    onSuccess: (res) => {
      setAssignMsg({ type: 'success', text: `Подписка назначена: ${res.login} до ${new Date(res.expires_at).toLocaleDateString('ru-RU')}` })
      setAssignLogin('')
    },
    onError: (e: Error) => {
      setAssignMsg({ type: 'error', text: e.message })
    },
  })

  const toggleBlockRealMoneyMutation = useMutation({
    mutationFn: (value: boolean) => adminApi.toggleBlockRealMoneyPurchases(value),
    onSuccess: () => {
      setShowConfirmModal(false)
    },
  })

  if (isLoading || settingsLoading) return <PageSpinner />
  if (isError) return <Alert variant="error" message="Не удалось загрузить статистику" />

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-slate-100">Обзор платформы</h1>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <StatCard
          label="Всего пользователей"
          value={data?.total_users ?? 0}
          icon={<Icon name="users" size={28} />}
        />
        <StatCard
          label="Активные подписки"
          value={data?.active_subscriptions ?? 0}
          icon={<Icon name="shield" size={28} />}
        />
        <StatCard
          label="Общая выручка"
          value={formatRubles(data?.total_revenue_kopecks ?? 0)}
          icon={<Icon name="coins" size={28} />}
        />
        <StatCard
          label="Ожидающие начисления"
          value={formatYAD(data?.pending_rewards ?? 0)}
          sub="Отложенные реферальные начисления"
          icon={<Icon name="tag" size={28} />}
        />
        <StatCard
          label="Открытые тикеты"
          value={data?.open_tickets ?? 0}
          sub={data?.open_tickets ? 'Требуют внимания' : 'Всё решено'}
          icon={<Icon name="message" size={28} />}
        />
        <StatCard
          label="Рискованные пользователи"
          value={data?.high_risk_users ?? 0}
          sub="Уровень риска ≥ 70"
          icon={<Icon name="chart" size={28} />}
        />
      </div>

      <Card title="Быстрые ссылки">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[
            { href: '/admin/users',   iconName: 'users'   as const, label: 'Пользователи' },
            { href: '/admin/tickets', iconName: 'message' as const, label: 'Тикеты' },
            { href: '/admin/promo',   iconName: 'tag'     as const, label: 'Промокоды' },
            { href: '/dashboard',     iconName: 'back'    as const, label: 'Личный кабинет' },
          ].map((item) => (
            <Link
              key={item.href}
              to={item.href}
              className="flex flex-col items-center gap-2 rounded-xl border border-surface-700 bg-surface-900 p-4 text-center hover:border-primary-900/60 hover:bg-primary-500/5 transition-colors"
            >
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary-500/10 text-primary-500">
                <Icon name={item.iconName} size={16} />
              </div>
              <span className="text-sm font-medium text-slate-300">{item.label}</span>
            </Link>
          ))}
        </div>
      </Card>

      <Card title="Назначить подписку">
        <div className="space-y-4">
          <p className="text-sm text-slate-400">
            Вручную активировать или продлить подписку пользователю по логину.
          </p>
          {assignMsg && (
            <Alert variant={assignMsg.type} message={assignMsg.text} />
          )}
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="flex-1">
              <label className="mb-1 block text-xs font-medium text-slate-400">Логин пользователя</label>
              <Input
                placeholder="username"
                value={assignLogin}
                onChange={(e) => setAssignLogin(e.target.value)}
              />
            </div>
            <div className="w-full sm:w-40">
              <label className="mb-1 block text-xs font-medium text-slate-400">Тариф</label>
              <select
                value={assignPlan}
                onChange={(e) => setAssignPlan(e.target.value)}
                className="w-full rounded-lg border border-surface-600 bg-surface-800 px-3 py-2 text-sm text-slate-200 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
              >
                {PLAN_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            </div>
            <Button
              loading={assignMutation.isPending}
              disabled={!assignLogin.trim()}
              onClick={() => assignMutation.mutate()}
              className="shrink-0"
            >
              Назначить
            </Button>
          </div>
        </div>
      </Card>

      <Card title="Настройки платформы">
        <div className="space-y-4">
          <div className="flex items-center justify-between rounded-lg border border-surface-700 bg-surface-900/50 p-4">
            <div className="flex-1">
              <h3 className="font-medium text-slate-100">Заблокировать покупки за реальные деньги</h3>
              <p className="text-sm text-slate-400 mt-1">
                {settings?.block_real_money_purchases
                  ? '🔴 Активно — платежи за рубли отключены'
                  : '🟢 Неактивно — платежи за рубли разрешены'}
              </p>
            </div>
            <button
              onClick={() => setShowConfirmModal(true)}
              className={`ml-4 relative inline-flex h-8 w-16 items-center rounded-full transition-colors ${
                settings?.block_real_money_purchases
                  ? 'bg-red-500/20 border border-red-500/50'
                  : 'bg-green-500/20 border border-green-500/50'
              }`}
            >
              <span
                className={`inline-block h-6 w-6 transform rounded-full bg-white transition-transform ${
                  settings?.block_real_money_purchases ? 'translate-x-9' : 'translate-x-1'
                }`}
              />
            </button>
          </div>
        </div>
      </Card>

      <Modal
        open={showConfirmModal}
        onClose={() => setShowConfirmModal(false)}
        title={settings?.block_real_money_purchases ? 'Разрешить покупки за деньги?' : 'Заблокировать покупки за деньги?'}
        footer={
          <div className="flex gap-2">
            <Button
              variant="secondary"
              onClick={() => setShowConfirmModal(false)}
            >
              Отмена
            </Button>
            <Button
              variant={settings?.block_real_money_purchases ? 'success' : 'danger'}
              loading={toggleBlockRealMoneyMutation.isPending}
              onClick={() => toggleBlockRealMoneyMutation.mutate(!settings?.block_real_money_purchases)}
            >
              {settings?.block_real_money_purchases ? 'Разрешить' : 'Заблокировать'}
            </Button>
          </div>
        }
      >
        <div className="space-y-4">
          <p className="text-sm text-slate-300">
            {settings?.block_real_money_purchases
              ? 'Вы собираетесь разрешить покупки подписок за реальные деньги. Платежи станут доступны.'
              : 'Вы собираетесь заблокировать все платежи за реальные деньги. Пользователи смогут покупать подписку только за ЯД.'}
          </p>
          {settings?.block_real_money_purchases && (
            <Alert variant="success" message="✅ Платежи будут восстановлены" />
          )}
          {!settings?.block_real_money_purchases && (
            <Alert variant="warning" message="⚠️ Это действие остановит все платежи за рубли" />
          )}
        </div>
      </Modal>
    </div>
  )
}
