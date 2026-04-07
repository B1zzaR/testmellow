import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { subscriptionsApi } from '@/api/subscriptions'
import { promoApi } from '@/api/promo'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { Icon } from '@/components/ui/Icons'
import type { SubscriptionPlan } from '@/api/types'
import { planLabel } from '@/utils/formatters'

const PLANS: { key: SubscriptionPlan; label: string; price: number; days: number; popular?: boolean }[] = [
  { key: '1week',   label: '1 неделя',   price: 40,  days: 7              },
  { key: '1month',  label: '1 месяц',  price: 100, days: 30, popular: true },
  { key: '3months', label: '3 месяца', price: 270, days: 90             },
]

function discountedPrice(price: number, percent: number): number {
  return Math.max(0, Math.round(price * (1 - percent / 100)))
}

export function RenewalPage() {
  const [selectedPlan, setSelectedPlan] = useState<SubscriptionPlan | null>(null)
  const [errorMsg, setErrorMsg] = useState('')
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: discountData } = useQuery({
    queryKey: ['active-discount'],
    queryFn: promoApi.getActiveDiscount,
  })

  const discount = discountData?.active_discount_percent ?? 0
  const discountCode = discountData?.active_discount_code ?? ''

  const renewMutation = useMutation({
    mutationFn: (plan: SubscriptionPlan) =>
      subscriptionsApi.renew({ plan, return_url: window.location.href }),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['subscriptions'] })
      queryClient.invalidateQueries({ queryKey: ['active-discount'] })
      if (res.redirect_url && res.redirect_url !== window.location.href) {
        window.location.href = res.redirect_url
      } else {
        navigate('/subscriptions')
      }
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div className="flex items-center gap-3">
        <button
          onClick={() => navigate(-1)}
          className="flex h-9 w-9 items-center justify-center rounded-lg border border-gray-200 dark:border-surface-700 text-gray-500 dark:text-slate-500 hover:bg-gray-100 dark:hover:bg-surface-700 transition-colors"
        >
          <Icon name="back" size={16} />
        </button>
        <div>
          <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">Продлить подписку</h1>
          <p className="text-sm text-gray-500 dark:text-slate-500">Продление добавляет дни с текущей даты истечения</p>
        </div>
      </div>

      {errorMsg && <Alert variant="error" message={errorMsg} />}

      {/* Active discount banner */}
      {discount > 0 && (
        <div className="flex items-center gap-3 rounded-xl border border-primary-900/40 bg-primary-500/5 px-4 py-3">
          <Icon name="tag" size={16} className="shrink-0 text-primary-500" />
          <div className="min-w-0">
            <p className="text-sm font-semibold text-primary-500">
              Скидка {discount}% активна
            </p>
            <p className="mt-0.5 text-xs text-gray-500 dark:text-slate-500">
              Промокод <span className="font-mono font-bold text-gray-700 dark:text-slate-300">{discountCode}</span> будет применён при оплате
            </p>
          </div>
        </div>
      )}

      <Card title="Выбрать тариф">
        <div className="grid gap-4 sm:grid-cols-3">
          {PLANS.map((plan) => {
            const isSelected = selectedPlan === plan.key
            const finalPrice = discountedPrice(plan.price, discount)
            const isFree = finalPrice === 0
            return (
              <button
                key={plan.key}
                onClick={() => setSelectedPlan(plan.key)}
                className={[
                  'relative rounded-xl border-2 p-5 text-left transition-all',
                  isSelected
                    ? 'border-primary-500 bg-primary-500/5 dark:bg-primary-500/10'
                    : 'border-gray-200 dark:border-surface-600 hover:border-gray-300 dark:hover:border-surface-500',
                ].join(' ')}
              >
                {plan.popular && (
                  <span className="absolute right-3 top-3 rounded-full bg-primary-500/15 px-2 py-0.5 text-[10px] font-semibold text-primary-500">
                    Популярный
                  </span>
                )}
                <p className="text-base font-bold text-gray-900 dark:text-slate-100">{plan.label}</p>

                {discount > 0 ? (
                  <div className="mt-2">
                    <span className="text-sm text-gray-400 dark:text-slate-600 line-through">{plan.price} ₽</span>
                    <p className={`text-2xl font-extrabold ${isFree ? 'text-green-500' : 'text-primary-500'}`}>
                      {isFree ? 'Бесплатно' : `${finalPrice} ₽`}
                    </p>
                  </div>
                ) : (
                  <p className="mt-2 text-2xl font-extrabold text-primary-500">{plan.price} ₽</p>
                )}

                <p className="mt-1.5 text-xs text-gray-500 dark:text-slate-500">{plan.days} дней</p>
              </button>
            )
          })}
        </div>

        {selectedPlan && (
          <div className="mt-4 flex items-center gap-2 rounded-lg bg-primary-500/5 dark:bg-primary-500/10 border border-primary-900/30 px-4 py-3 text-sm text-primary-600 dark:text-primary-400">
            <Icon name="check" size={14} />
            {(() => {
              const plan = PLANS.find(p => p.key === selectedPlan)!
              const finalPrice = discountedPrice(plan.price, discount)
              return (
                <span>
                  Выбран: <strong>{planLabel(selectedPlan)}</strong>
                  {discount > 0 && (
                    <> — итого{' '}
                      <strong>{finalPrice === 0 ? 'бесплатно' : `${finalPrice} ₽`}</strong>
                      {' '}(скидка {discount}%)
                    </>
                  )}
                  {(finalPrice > 0) && ' — вы будете перенаправлены на страницу оплаты.'}
                  {(finalPrice === 0) && ' — подписка активируется сразу.'}
                </span>
              )
            })()}
          </div>
        )}

        <div className="mt-5 flex gap-3">
          <Button
            disabled={!selectedPlan}
            loading={renewMutation.isPending}
            onClick={() => selectedPlan && renewMutation.mutate(selectedPlan)}
          >
            {selectedPlan && discountedPrice(PLANS.find(p => p.key === selectedPlan)!.price, discount) === 0
              ? 'Активировать бесплатно'
              : <><Icon name="external" size={14} />Перейти к оплате</>
            }
          </Button>
          <Button variant="secondary" onClick={() => navigate(-1)}>Отмена</Button>
        </div>
      </Card>
    </div>
  )
}
