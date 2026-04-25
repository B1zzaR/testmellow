import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { QRCodeSVG } from 'qrcode.react'
import { subscriptionsApi } from '@/api/subscriptions'
import { devicesApi } from '@/api/devices'
import { promoApi } from '@/api/promo'
import { profileApi } from '@/api/profile'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { PageSpinner } from '@/components/ui/Spinner'
import { subscriptionStatusBadge } from '@/components/ui/Badge'
import { Icon } from '@/components/ui/Icons'
import { formatDate, formatRubles, planLabel, daysUntil, formatBytes } from '@/utils/formatters'
import { PendingPayments } from '@/components/PendingPayments'
import { DeviceList } from '@/components/DeviceList'
import { SubscriptionDetails } from '@/components/SubscriptionDetails'
import type { SubscriptionPlan } from '@/api/types'

function discountedPrice(price: number, percent: number): number {
  return Math.max(0, Math.round(price * (1 - percent / 100)))
}

function ConnectionBlock() {
  const [copied, setCopied] = useState(false)
  const [showQR, setShowQR] = useState(false)
  const { data, isLoading } = useQuery({
    queryKey: ['connection'],
    queryFn: profileApi.getConnection,
    retry: 2,
    retryDelay: 3000,
  })

  if (isLoading || !data?.subscribe_url) return null

  const url = data.subscribe_url

  const copy = () => {
    navigator.clipboard.writeText(url).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div className="mt-5 border-t border-gray-100 dark:border-surface-700 pt-4">
      <p className="mb-2 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-slate-600">
        Ссылка для подключения
      </p>
      <div className="flex items-center gap-2 rounded-lg border border-gray-200 dark:border-surface-600 bg-gray-50 dark:bg-surface-800 px-3 py-2">
        <span className="flex-1 truncate font-mono text-xs text-gray-600 dark:text-slate-400">{url}</span>
        <button
          onClick={() => setShowQR((v) => !v)}
          title="Показать QR-код"
          className={`flex shrink-0 items-center justify-center rounded-md border px-2 py-1 text-xs transition-all active:scale-95 ${
            showQR
              ? 'border-primary-500/50 bg-primary-500/10 text-primary-400'
              : 'border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-700 text-gray-500 dark:text-slate-400 hover:bg-gray-50 dark:hover:bg-surface-600'
          }`}
        >
          <Icon name="smartphone" size={14} />
        </button>
        <button
          onClick={copy}
          className="flex shrink-0 items-center gap-1.5 rounded-md border border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-700 px-2.5 py-1 text-xs font-medium text-gray-700 dark:text-slate-300 hover:bg-gray-50 dark:hover:bg-surface-600 active:scale-95 transition-all"
        >
          <Icon name={copied ? 'check' : 'copy'} size={12} className={copied ? 'text-primary-500' : ''} />
          <span className="hidden sm:inline whitespace-nowrap">{copied ? 'Скопировано' : 'Скопировать'}</span>
        </button>
      </div>

      {showQR && (
        <div className="mt-3 flex justify-center">
          <div className="rounded-xl border border-gray-200 dark:border-surface-600 bg-white dark:bg-surface-800 p-4">
            <QRCodeSVG
              value={url}
              size={180}
              bgColor="transparent"
              fgColor="currentColor"
              className="text-gray-900 dark:text-slate-100"
              level="M"
            />
            <p className="mt-2 text-center text-[10px] text-gray-400 dark:text-slate-600">Сканируйте QR в приложении</p>
          </div>
        </div>
      )}

      <p className="mt-1.5 text-xs text-gray-400 dark:text-slate-600">
        Вставьте в Happ, V2RayN, Hiddify, Streisand или любой совместимый клиент.
      </p>
    </div>
  )
}

const PLANS: { key: SubscriptionPlan; label: string; price: number; days: number; yadBonus: number; popular?: boolean }[] = [
  { key: '1week',   label: '1 неделя',  price: 40,  days: 7,  yadBonus: 10              },
  { key: '1month',  label: '1 месяц',   price: 100, days: 30, yadBonus: 25, popular: true },
  { key: '3months', label: '3 месяца',  price: 270, days: 90, yadBonus: 75              },
]

export function SubscriptionsPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['subscriptions'],
    queryFn: subscriptionsApi.list,
  })
  const { data: discountData } = useQuery({
    queryKey: ['active-discount'],
    queryFn: promoApi.getActiveDiscount,
  })
  const { data: devicesData } = useQuery({
    queryKey: ['devices'],
    queryFn: devicesApi.list,
  })
  const { data: trafficData } = useQuery({
    queryKey: ['traffic'],
    queryFn: profileApi.getTraffic,
  })
  const queryClient = useQueryClient()
  const discount = discountData?.active_discount_percent ?? 0
  const discountCode = discountData?.active_discount_code ?? ''
  const [selectedPlan, setSelectedPlan] = useState<SubscriptionPlan | null>(null)
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')

  const buyMutation = useMutation({
    mutationFn: (plan: SubscriptionPlan) =>
      subscriptionsApi.buy({ plan, return_url: window.location.href }),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['subscriptions'] })
      queryClient.invalidateQueries({ queryKey: ['active-discount'] })
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      queryClient.invalidateQueries({ queryKey: ['pendingPayments'] })
      if (res.redirect_url) {
        // Open Platega in a new tab so the user stays on this page
        // and sees the pending payment card with the countdown.
        window.open(res.redirect_url, '_blank', 'noopener,noreferrer')
      } else {
        setSuccessMsg('Подписка активирована!')
      }
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const trialMutation = useMutation({
    mutationFn: subscriptionsApi.activateTrial,
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['subscriptions'] })
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      setSuccessMsg(`Пробный период активирован! Истекает ${new Date(res.expires_at).toLocaleDateString('ru-RU')}`)
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  if (isLoading) return <PageSpinner />

  const subs = data?.subscriptions ?? []
  const activeSub = subs.find((s) => s.status === 'active' || s.status === 'trial')

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">Подписки</h1>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">Управляйте планами VPN-доступа</p>
      </div>

      {successMsg && <Alert variant="success" message={successMsg} />}
      {errorMsg && <Alert variant="error" message={errorMsg} />}

      <PendingPayments
        onPaymentConfirmed={() => {
          queryClient.invalidateQueries({ queryKey: ['subscriptions'] })
          queryClient.invalidateQueries({ queryKey: ['profile'] })
        }}
      />

      {/* Active subscription */}
      {activeSub && (
        <Card title={daysUntil(activeSub.expires_at) > 0 ? 'Активная подписка' : 'Подписка закончилась'} glow>
          {daysUntil(activeSub.expires_at) === 0 && (
            <div className="mb-4 flex items-center gap-3 rounded-xl border border-red-500/30 bg-red-500/5 px-4 py-3">
              <span className="shrink-0 text-red-500">⏰</span>
              <p className="text-sm text-red-600 dark:text-red-400">
                Подписка закончилась — продлите для продолжения доступа.
              </p>
            </div>
          )}
          {daysUntil(activeSub.expires_at) > 0 && daysUntil(activeSub.expires_at) <= 7 && (
            <div className="mb-4 flex items-center gap-3 rounded-xl border border-yellow-500/30 bg-yellow-500/5 px-4 py-3">
              <span className="shrink-0 text-yellow-500">⚠</span>
              <p className="text-sm text-yellow-600 dark:text-yellow-400">
                Подписка истекает через <strong>{daysUntil(activeSub.expires_at)} дн.</strong> — не забудьте продлить.
              </p>
            </div>
          )}
          <div className="grid grid-cols-2 gap-3 sm:flex sm:flex-wrap sm:gap-6">
            {[
              { label: 'Тариф',        value: planLabel(activeSub.plan) },
              { label: 'Осталось дней',   value: daysUntil(activeSub.expires_at) > 0 ? `${daysUntil(activeSub.expires_at)} дней` : 'Подписка закончилась' },
              { label: 'Истекает',     value: formatDate(activeSub.expires_at) },
              ...(activeSub.paid_kopecks > 0 ? [{ label: 'Оплачено', value: formatRubles(activeSub.paid_kopecks) }] : []),
            ].map(({ label, value }) => (
              <div key={label}>
                <p className="text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:text-slate-600">{label}</p>
                <p className="mt-1 text-sm font-semibold text-gray-900 dark:text-slate-100">{value}</p>
              </div>
            ))}
            <div>
              <p className="text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:text-slate-600">Статус</p>
              <div className="mt-1">{subscriptionStatusBadge(activeSub.status)}</div>
            </div>
          </div>

          {/* Traffic usage */}
          {trafficData && (
            <div className="mt-5 border-t border-gray-100 dark:border-surface-700 pt-4">
              <p className="mb-2 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-slate-600">
                Трафик
              </p>
              <div className="flex items-baseline gap-2">
                <span className="text-sm font-semibold text-gray-900 dark:text-slate-100">
                  {formatBytes(trafficData.used_bytes)}
                </span>
                {trafficData.limit_bytes > 0 && (
                  <span className="text-xs text-gray-400 dark:text-slate-500">
                    / {formatBytes(trafficData.limit_bytes)}
                  </span>
                )}
                {trafficData.limit_bytes === 0 && (
                  <span className="text-xs text-gray-400 dark:text-slate-500">/ ∞</span>
                )}
              </div>
              {trafficData.limit_bytes > 0 && (
                <div className="mt-2 h-2 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-surface-700">
                  <div
                    className="h-full rounded-full bg-primary-500 transition-all"
                    style={{ width: `${Math.min(100, (trafficData.used_bytes / trafficData.limit_bytes) * 100)}%` }}
                  />
                </div>
              )}
            </div>
          )}

          <ConnectionBlock />

          {/* Subscription periods details */}
          <SubscriptionDetails
            allSubscriptions={subs}
            totalDays={daysUntil(activeSub.expires_at)}
          />
        </Card>
      )}

      {/* Devices */}
      {activeSub && devicesData && <DeviceList data={devicesData} isTrial={activeSub.status === 'trial'} />}

      {/* Plan selector */}
      <Card title={activeSub ? 'Продлить подписку' : 'Выбрать тариф'}>

        {/* Active discount banner */}
        {discount > 0 && (
          <div className="mb-4 flex items-center gap-3 rounded-xl border border-primary-900/40 bg-primary-500/5 px-4 py-3">
            <Icon name="tag" size={16} className="shrink-0 text-primary-500" />
            <div className="min-w-0">
              <p className="text-sm font-semibold text-primary-500">Скидка {discount}% активна</p>
              <p className="mt-0.5 text-xs text-gray-500 dark:text-slate-500">
                Промокод <span className="font-mono font-bold text-gray-700 dark:text-slate-300">{discountCode}</span> будет применён при оплате
              </p>
            </div>
          </div>
        )}

        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
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
                <p className="mt-1.5 text-xs text-gray-500 dark:text-slate-500 flex items-center gap-1">
                  <Icon name="check" size={10} className="text-primary-500" />
                  {plan.days} дней доступа
                </p>
                <p className="mt-1 flex items-center gap-1 text-xs font-semibold text-yellow-500">
                  <Icon name="coins" size={10} />
                  +{plan.yadBonus} ЯД бонус
                </p>
              </button>
            )
          })}
        </div>

        <div className="mt-5 flex flex-wrap gap-3">
          <Button
            disabled={!selectedPlan}
            loading={buyMutation.isPending}
            onClick={() => selectedPlan && buyMutation.mutate(selectedPlan)}
          >
            {selectedPlan && discountedPrice(PLANS.find(p => p.key === selectedPlan)!.price, discount) === 0
              ? 'Активировать бесплатно'
              : activeSub
                ? <><Icon name="external" size={14} />Продлить</>
                : <><Icon name="external" size={14} />Оплатить</>
            }
          </Button>

          {!activeSub && (
            <Button
              variant="secondary"
              loading={trialMutation.isPending}
              onClick={() => trialMutation.mutate()}
            >
              Активировать пробный период
            </Button>
          )}
        </div>
      </Card>

      {/* History */}
      {subs.length > 0 && (
        <Card title="История">
          <div className="space-y-2">
            {subs.map((sub) => (
              <div
                key={sub.id}
                className="flex items-center justify-between rounded-lg border border-gray-100 dark:border-surface-700 px-4 py-3 hover:bg-gray-50 dark:hover:bg-surface-800 transition-colors"
              >
                <div>
                  <span className="font-medium text-gray-800 dark:text-slate-200">{planLabel(sub.plan)}</span>
                  <p className="mt-0.5 text-xs text-gray-400 dark:text-slate-600">
                    {formatDate(sub.starts_at)} – {formatDate(sub.expires_at)}
                  </p>
                </div>
                <div className="flex items-center gap-3">
                  {sub.paid_kopecks > 0 && (
                    <span className="text-sm text-gray-500 dark:text-slate-500">{formatRubles(sub.paid_kopecks)}</span>
                  )}
                  {subscriptionStatusBadge(sub.status)}
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}
    </div>
  )
}

