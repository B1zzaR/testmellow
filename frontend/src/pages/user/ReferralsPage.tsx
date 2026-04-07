import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { referralsApi } from '@/api/referrals'
import { Card, StatCard } from '@/components/ui/Card'
import { Alert } from '@/components/ui/Alert'
import { PageSpinner } from '@/components/ui/Spinner'
import { formatDate, formatYAD, formatRubles } from '@/utils/formatters'

export function ReferralsPage() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['referrals'],
    queryFn: referralsApi.list,
  })
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    if (data?.referral_code) {
      navigator.clipboard.writeText(data.referral_code)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  if (isLoading) return <PageSpinner />
  if (isError) return <Alert variant="error" message="Failed to load referrals" />

  const referrals = data?.referrals ?? []

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Referrals</h1>

      <div className="grid gap-4 sm:grid-cols-2">
        <StatCard
          label="Total Referrals"
          value={data?.referral_count ?? 0}
          icon="👥"
        />
        <StatCard
          label="Total Reward Earned"
          value={formatYAD(referrals.reduce((sum, r) => sum + r.total_reward, 0))}
          icon="💎"
        />
      </div>

      <Card title="Your Referral Code">
        <p className="mb-3 text-sm text-gray-500">
          Share this code with friends. They enter it during registration. You earn YAD rewards when they subscribe.
        </p>
        <div className="flex items-center gap-2">
          <input
            readOnly
            value={data?.referral_code ?? ''}
            className="flex-1 rounded-lg border border-gray-300 bg-gray-50 px-3 py-2.5 text-sm font-mono tracking-widest text-gray-700"
          />
          <button
            onClick={handleCopy}
            className="rounded-lg border border-gray-300 bg-white px-4 py-2.5 text-sm font-medium hover:bg-gray-50"
          >
            {copied ? '✓ Copied' : 'Copy'}
          </button>
        </div>
        <p className="mt-2 text-xs text-gray-400">
          You earn 30% immediately + 70% after 30 days of each referral payment.
        </p>
      </Card>

      {referrals.length > 0 && (
        <Card title="Referred Users">
          <div className="space-y-2">
            {referrals.map((ref) => (
              <div
                key={ref.id}
                className="flex items-center justify-between rounded-lg border border-gray-100 px-4 py-3"
              >
                <div>
                  <p className="text-sm font-medium text-gray-700">
                    User {ref.referee_id.slice(0, 8)}…
                  </p>
                  <p className="text-xs text-gray-400">Since {formatDate(ref.created_at)}</p>
                </div>
                <div className="text-right">
                  <p className="text-sm font-semibold text-gray-900">
                    {formatYAD(ref.total_reward)} earned
                  </p>
                  <p className="text-xs text-gray-400">
                    LTV: {formatRubles(ref.total_paid_ltv)}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}

      {referrals.length === 0 && (
        <div className="rounded-xl border border-dashed border-gray-300 px-6 py-10 text-center">
          <p className="text-sm text-gray-400">No referrals yet. Share your code to invite friends!</p>
        </div>
      )}
    </div>
  )
}
