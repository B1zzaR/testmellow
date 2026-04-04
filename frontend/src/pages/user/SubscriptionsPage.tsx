import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { subscriptionsApi } from '@/api/subscriptions'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { PageSpinner } from '@/components/ui/Spinner'
import { subscriptionStatusBadge } from '@/components/ui/Badge'
import { formatDate, formatRubles, planLabel, daysUntil } from '@/utils/formatters'
import { PendingPayments } from '@/components/PendingPayments'
import type { SubscriptionPlan } from '@/api/types'

const PLANS: { key: SubscriptionPlan; label: string; price: number; days: number }[] = [
  { key: '1week', label: '1 Week', price: 40, days: 7 },
  { key: '1month', label: '1 Month', price: 100, days: 30 },
  { key: '3months', label: '3 Months', price: 270, days: 90 },
]

export function SubscriptionsPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['subscriptions'],
    queryFn: subscriptionsApi.list,
  })
  const queryClient = useQueryClient()
  const [selectedPlan, setSelectedPlan] = useState<SubscriptionPlan | null>(null)
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')

  const buyMutation = useMutation({
    mutationFn: (plan: SubscriptionPlan) =>
      subscriptionsApi.buy({ plan, return_url: window.location.href }),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['subscriptions'] })
      window.location.href = res.redirect_url
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const trialMutation = useMutation({
    mutationFn: subscriptionsApi.activateTrial,
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['subscriptions'] })
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      setSuccessMsg(`Trial activated! Expires ${new Date(res.expires_at).toLocaleDateString()}`)
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  if (isLoading) return <PageSpinner />

  const subs = data?.subscriptions ?? []
  const activeSub = subs.find((s) => s.status === 'active' || s.status === 'trial')

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Subscriptions</h1>

      {successMsg && <Alert variant="success" message={successMsg} />}
      {errorMsg && <Alert variant="error" message={errorMsg} />}

      {/* Pending payments – auto-polls every 15s */}
      <PendingPayments
        onPaymentConfirmed={() => {
          queryClient.invalidateQueries({ queryKey: ['subscriptions'] })
          queryClient.invalidateQueries({ queryKey: ['profile'] })
        }}
      />

      {/* Active subscription */}
      {activeSub && (
        <Card title="Active Subscription">
          <div className="flex flex-wrap gap-6">
            <div>
              <p className="text-xs text-gray-500">Plan</p>
              <p className="mt-0.5 font-semibold">{planLabel(activeSub.plan)}</p>
            </div>
            <div>
              <p className="text-xs text-gray-500">Status</p>
              <div className="mt-0.5">{subscriptionStatusBadge(activeSub.status)}</div>
            </div>
            <div>
              <p className="text-xs text-gray-500">Expires</p>
              <p className="mt-0.5 font-semibold">{formatDate(activeSub.expires_at)}</p>
            </div>
            <div>
              <p className="text-xs text-gray-500">Days Remaining</p>
              <p className="mt-0.5 font-semibold">{daysUntil(activeSub.expires_at)}</p>
            </div>
            {activeSub.paid_kopecks > 0 && (
              <div>
                <p className="text-xs text-gray-500">Paid</p>
                <p className="mt-0.5 font-semibold">{formatRubles(activeSub.paid_kopecks)}</p>
              </div>
            )}
          </div>
        </Card>
      )}

      {/* Buy / Trial */}
      <Card title="Buy a Plan">
        <div className="grid gap-4 sm:grid-cols-3">
          {PLANS.map((plan) => (
            <button
              key={plan.key}
              onClick={() => setSelectedPlan(plan.key)}
              className={`rounded-xl border-2 p-5 text-left transition-all ${
                selectedPlan === plan.key
                  ? 'border-primary-500 bg-primary-50'
                  : 'border-gray-200 hover:border-gray-300'
              }`}
            >
              <p className="text-lg font-bold text-gray-900">{plan.label}</p>
              <p className="mt-1 text-2xl font-extrabold text-primary-600">{plan.price} ₽</p>
              <p className="mt-1 text-xs text-gray-500">{plan.days} days access</p>
            </button>
          ))}
        </div>

        <div className="mt-5 flex flex-wrap gap-3">
          <Button
            disabled={!selectedPlan}
            loading={buyMutation.isPending}
            onClick={() => selectedPlan && buyMutation.mutate(selectedPlan)}
          >
            Pay Now
          </Button>

          {!activeSub && (
            <Button
              variant="secondary"
              loading={trialMutation.isPending}
              onClick={() => trialMutation.mutate()}
            >
              Activate Free Trial
            </Button>
          )}
        </div>
      </Card>

      {/* History */}
      {subs.length > 0 && (
        <Card title="Subscription History">
          <div className="space-y-2">
            {subs.map((sub) => (
              <div
                key={sub.id}
                className="flex items-center justify-between rounded-lg border border-gray-100 px-4 py-3"
              >
                <div>
                  <span className="font-medium">{planLabel(sub.plan)}</span>
                  <span className="ml-3 text-xs text-gray-400">
                    {formatDate(sub.starts_at)} – {formatDate(sub.expires_at)}
                  </span>
                </div>
                <div className="flex items-center gap-3">
                  {sub.paid_kopecks > 0 && (
                    <span className="text-sm text-gray-600">{formatRubles(sub.paid_kopecks)}</span>
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
