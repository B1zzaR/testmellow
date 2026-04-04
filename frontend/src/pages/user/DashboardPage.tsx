import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { profileApi, balanceApi } from '@/api/profile'
import { subscriptionsApi } from '@/api/subscriptions'
import { ticketsApi } from '@/api/tickets'
import { StatCard, Card } from '@/components/ui/Card'
import { PageSpinner } from '@/components/ui/Spinner'
import { Button } from '@/components/ui/Button'
import { subscriptionStatusBadge } from '@/components/ui/Badge'
import { formatDate, formatYAD, daysUntil, planLabel } from '@/utils/formatters'

// ─── Connection link row (inline inside subscription card) ───────────────────

function ConnectionRow() {
  const [copied, setCopied] = useState(false)
  const { data, isLoading, isError } = useQuery({
    queryKey: ['connection'],
    queryFn: profileApi.getConnection,
    retry: false,
  })

  if (isLoading) return <p className="mt-4 text-xs text-gray-400">Loading connection link…</p>
  if (isError || !data?.subscribe_url) return null

  const url = data.subscribe_url

  const copy = () => {
    navigator.clipboard.writeText(url).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div className="mt-5 border-t border-gray-100 pt-4">
      <p className="mb-2 text-xs font-medium text-gray-500 uppercase tracking-wide">Connection link</p>
      <div className="flex items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2">
        <span className="flex-1 truncate font-mono text-xs text-gray-700">{url}</span>
        <button
          onClick={copy}
          className="shrink-0 rounded-md border border-gray-300 bg-white px-3 py-1 text-xs font-medium text-gray-700 hover:bg-gray-50 active:scale-95 transition-all"
        >
          {copied ? '✓ Copied' : 'Copy'}
        </button>
      </div>
      <p className="mt-1.5 text-xs text-gray-400">
        Paste into Happ, V2RayN, Hiddify, Streisand or any compatible client.
      </p>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export function DashboardPage() {
  const { data: profile, isLoading: profileLoading } = useQuery({
    queryKey: ['profile'],
    queryFn: profileApi.getProfile,
  })
  const { data: balance } = useQuery({
    queryKey: ['balance'],
    queryFn: balanceApi.getBalance,
  })
  const { data: subsData } = useQuery({
    queryKey: ['subscriptions'],
    queryFn: subscriptionsApi.list,
  })
  const { data: ticketsData } = useQuery({
    queryKey: ['tickets'],
    queryFn: ticketsApi.list,
  })

  if (profileLoading) return <PageSpinner />

  const activeSub = (subsData?.subscriptions ?? []).find((s) => s.status === 'active' || s.status === 'trial')
  const openTickets = (ticketsData?.tickets ?? []).filter((t) => t.status === 'open').length

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-gray-900">
          Welcome back{profile?.email ? `, ${profile.email}` : ''}
        </h1>
        <p className="mt-1 text-sm text-gray-500">Here's what's happening with your account</p>
      </div>

      {/* Stats row */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="YAD Balance"
          value={formatYAD(balance?.yad_balance ?? profile?.yad_balance ?? 0)}
          sub={`≈ ${(balance?.yad_ruble_value ?? 0).toFixed(2)} ₽`}
          icon="💎"
        />
        <StatCard
          label="Active Plan"
          value={activeSub ? planLabel(activeSub.plan) : 'None'}
          sub={activeSub ? `${daysUntil(activeSub.expires_at)} days left` : 'No active subscription'}
          icon="🛡"
        />
        <StatCard
          label="Open Tickets"
          value={openTickets}
          sub={openTickets > 0 ? 'Awaiting response' : 'All resolved'}
          icon="💬"
        />
        <StatCard
          label="Trial Status"
          value={profile?.trial_used ? 'Used' : 'Available'}
          sub={profile?.trial_used ? 'Buy a plan to get access' : 'Activate free trial'}
          icon="🎁"
        />
      </div>

      {/* Active subscription */}
      {activeSub && (
        <Card title="Active Subscription" action={
          <Link to="/subscriptions/renew">
            <Button size="sm" variant="secondary">Renew</Button>
          </Link>
        }>
          <div className="flex flex-wrap gap-6">
            <div>
              <p className="text-xs text-gray-500">Plan</p>
              <p className="mt-0.5 font-medium">{planLabel(activeSub.plan)}</p>
            </div>
            <div>
              <p className="text-xs text-gray-500">Status</p>
              <div className="mt-0.5">{subscriptionStatusBadge(activeSub.status)}</div>
            </div>
            <div>
              <p className="text-xs text-gray-500">Expires</p>
              <p className="mt-0.5 font-medium">{formatDate(activeSub.expires_at)}</p>
            </div>
            <div>
              <p className="text-xs text-gray-500">Days Left</p>
              <p className="mt-0.5 font-medium">{daysUntil(activeSub.expires_at)}</p>
            </div>
          </div>
          <ConnectionRow />
        </Card>
      )}

      {/* No subscription - show purchase prompt */}
      {!activeSub && (
        <Card title="Get VPN Access">
          <p className="text-sm text-gray-600">
            {profile?.trial_used
              ? 'Your trial has ended. Buy a plan to restore VPN access.'
              : "You don't have an active subscription yet."}
          </p>
          <div className="mt-4 flex gap-3">
            {!profile?.trial_used && (
              <Link to="/subscriptions">
                <Button size="sm" variant="secondary">Activate Trial</Button>
              </Link>
            )}
            <Link to="/subscriptions">
              <Button size="sm">Buy Subscription</Button>
            </Link>
          </div>
        </Card>
      )}

      {/* Quick actions */}
      <Card title="Quick Actions">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[
            { to: '/subscriptions', icon: '🛡', label: 'Subscriptions' },
            { to: '/referrals', icon: '👥', label: 'Referrals' },
            { to: '/shop', icon: '🛒', label: 'Shop' },
            { to: '/promo', icon: '🎁', label: 'Promo Code' },
          ].map((action) => (
            <Link
              key={action.to}
              to={action.to}
              className="flex flex-col items-center gap-2 rounded-lg border border-gray-200 p-4 text-center hover:bg-gray-50"
            >
              <span className="text-2xl">{action.icon}</span>
              <span className="text-sm font-medium text-gray-700">{action.label}</span>
            </Link>
          ))}
        </div>
      </Card>
    </div>
  )
}
