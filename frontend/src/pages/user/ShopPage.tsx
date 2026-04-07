import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { shopApi } from '@/api/shop'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { Modal } from '@/components/ui/Modal'
import { Icon } from '@/components/ui/Icons'
import { formatYAD } from '@/utils/formatters'
import type { SubscriptionPlan } from '@/api/types'

const SUB_PLANS: {
  key: SubscriptionPlan
  label: string
  yadPrice: number
  days: number
  popular?: boolean
  iconName: 'shield' | 'star' | 'crown'
  description: string
}[] = [
  {
    key: '1week',
    label: '1 неделя',
    yadPrice: 30,
    days: 7,
    iconName: 'shield',
    description: 'Попробуй без обязательств',
  },
  {
    key: '1month',
    label: '1 месяц',
    yadPrice: 75,
    days: 30,
    popular: true,
    iconName: 'star',
    description: 'Самый популярный выбор',
  },
  {
    key: '3months',
    label: '3 месяца',
    yadPrice: 210,
    days: 90,
    iconName: 'crown',
    description: 'Максимальная выгода',
  },
]

export function ShopPage() {
  const queryClient = useQueryClient()
  const [buySubPlan, setBuySubPlan] = useState<SubscriptionPlan | null>(null)
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')

  const buySubMutation = useMutation({
    mutationFn: (plan: SubscriptionPlan) => shopApi.buySubscription(plan),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['balance'] })
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      queryClient.invalidateQueries({ queryKey: ['subscriptions'] })
      setSuccessMsg('Подписка успешно активирована!')
      setBuySubPlan(null)
    },
    onError: (e: Error) => {
      setBuySubPlan(null)
      setErrorMsg(e.message)
    },
  })

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">Подписки за ЯД</h1>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">
          Активируй доступ к VPN, используя накопленные ЯД
        </p>
      </div>

      {successMsg && <Alert variant="success" message={successMsg} />}
      {errorMsg && <Alert variant="error" message={errorMsg} />}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        {SUB_PLANS.map((plan) => (
          <div
            key={plan.key}
            className={`relative flex flex-col rounded-xl border bg-white dark:bg-surface-900 p-5 shadow-sm dark:shadow-card transition-all ${
              plan.popular
                ? 'border-primary-500 dark:border-primary-500 ring-1 ring-primary-500/30'
                : 'border-gray-200 dark:border-surface-700 hover:border-gray-300 dark:hover:border-surface-600'
            }`}
          >
            {plan.popular && (
              <span className="absolute -top-3 left-1/2 -translate-x-1/2 whitespace-nowrap rounded-full bg-primary-500 px-3 py-0.5 text-[11px] font-semibold text-white shadow">
                Популярный
              </span>
            )}

            <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-primary-500/10 text-primary-500">
              <Icon name={plan.iconName} size={20} />
            </div>

            <p className="mt-3 text-base font-bold text-gray-900 dark:text-slate-100">{plan.label}</p>
            <p className="mt-0.5 text-xs text-gray-400 dark:text-slate-500">{plan.description}</p>

            <p className="mt-4 text-3xl font-extrabold text-primary-500">{formatYAD(plan.yadPrice)}</p>
            <p className="mt-1 text-xs text-gray-400 dark:text-slate-500">{plan.days} дней доступа</p>

            <Button
              className="mt-5 w-full"
              size="sm"
              variant={plan.popular ? 'primary' : 'secondary'}
              onClick={() => setBuySubPlan(plan.key)}
            >
              Активировать
            </Button>
          </div>
        ))}
      </div>

      <Modal
        open={Boolean(buySubPlan)}
        onClose={() => setBuySubPlan(null)}
        title="Подтверждение покупки"
        footer={
          <>
            <Button variant="secondary" onClick={() => setBuySubPlan(null)}>
              Отмена
            </Button>
            <Button
              loading={buySubMutation.isPending}
              onClick={() => buySubPlan && buySubMutation.mutate(buySubPlan)}
            >
              Подтвердить
            </Button>
          </>
        }
      >
        {buySubPlan &&
          (() => {
            const plan = SUB_PLANS.find((p) => p.key === buySubPlan)!
            return (
              <>
                <p className="text-sm text-gray-600 dark:text-slate-400">
                  Активировать подписку{' '}
                  <strong className="text-gray-900 dark:text-slate-100">{plan.label}</strong> за{'  '}
                  <strong className="text-primary-500">{formatYAD(plan.yadPrice)}</strong>?
                </p>
                <p className="mt-2 text-xs text-gray-400 dark:text-slate-600">
                  Сумма будет списана с баланса ЯД. Если активная подписка есть — срок продлится.
                </p>
              </>
            )
          })()}
      </Modal>
    </div>
  )
}
