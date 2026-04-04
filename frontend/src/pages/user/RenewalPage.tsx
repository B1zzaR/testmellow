import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { subscriptionsApi } from '@/api/subscriptions'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import type { SubscriptionPlan } from '@/api/types'
import { planLabel } from '@/utils/formatters'

const PLANS: { key: SubscriptionPlan; label: string; price: number; days: number }[] = [
  { key: '1week', label: '1 Week', price: 40, days: 7 },
  { key: '1month', label: '1 Month', price: 100, days: 30 },
  { key: '3months', label: '3 Months', price: 270, days: 90 },
]

export function RenewalPage() {
  const [selectedPlan, setSelectedPlan] = useState<SubscriptionPlan | null>(null)
  const [errorMsg, setErrorMsg] = useState('')
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const renewMutation = useMutation({
    mutationFn: (plan: SubscriptionPlan) =>
      subscriptionsApi.renew({ plan, return_url: window.location.href }),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['subscriptions'] })
      window.location.href = res.redirect_url
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div className="flex items-center gap-3">
        <button
          onClick={() => navigate(-1)}
          className="rounded-lg p-2 text-gray-500 hover:bg-gray-100"
        >
          ←
        </button>
        <h1 className="text-2xl font-bold text-gray-900">Renew Subscription</h1>
      </div>

      {errorMsg && <Alert variant="error" message={errorMsg} />}

      <Card title="Select Plan to Renew">
        <p className="mb-4 text-sm text-gray-500">
          Renewing before expiry will extend your current subscription from the expiry date.
        </p>
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
              <p className="mt-1 text-xs text-gray-500">{plan.days} days</p>
            </button>
          ))}
        </div>

        {selectedPlan && (
          <div className="mt-4 rounded-lg bg-gray-50 p-4 text-sm text-gray-600">
            You selected: <strong>{planLabel(selectedPlan)}</strong>. You will be redirected to
            payment.
          </div>
        )}

        <div className="mt-5 flex gap-3">
          <Button
            disabled={!selectedPlan}
            loading={renewMutation.isPending}
            onClick={() => selectedPlan && renewMutation.mutate(selectedPlan)}
          >
            Proceed to Payment
          </Button>
          <Button variant="secondary" onClick={() => navigate(-1)}>
            Cancel
          </Button>
        </div>
      </Card>
    </div>
  )
}
