import { useQuery } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import { StatCard, Card } from '@/components/ui/Card'
import { PageSpinner } from '@/components/ui/Spinner'
import { Alert } from '@/components/ui/Alert'
import { formatRubles, formatYAD } from '@/utils/formatters'

export function AdminDashboardPage() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['admin-analytics'],
    queryFn: adminApi.getAnalytics,
    refetchInterval: 30_000,
  })

  if (isLoading) return <PageSpinner />
  if (isError) return <Alert variant="error" message="Failed to load analytics" />

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Platform Overview</h1>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <StatCard
          label="Total Users"
          value={data?.total_users ?? 0}
          icon="👤"
        />
        <StatCard
          label="Active Subscriptions"
          value={data?.active_subscriptions ?? 0}
          icon="🛡"
        />
        <StatCard
          label="Total Revenue"
          value={formatRubles(data?.total_revenue_kopecks ?? 0)}
          icon="💰"
        />
        <StatCard
          label="Pending Rewards"
          value={formatYAD(data?.pending_rewards ?? 0)}
          sub="Deferred referral rewards"
          icon="⏳"
        />
        <StatCard
          label="Open Tickets"
          value={data?.open_tickets ?? 0}
          sub={data?.open_tickets ? 'Need attention' : 'All resolved'}
          icon="🎫"
        />
        <StatCard
          label="High Risk Users"
          value={data?.high_risk_users ?? 0}
          sub="Risk score ≥ 70"
          icon="⚠️"
        />
      </div>

      <Card title="Quick Links">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[
            { href: '/admin/users', icon: '👤', label: 'Users' },
            { href: '/admin/tickets', icon: '🎫', label: 'Tickets' },
            { href: '/admin/promo', icon: '🏷', label: 'Promo Codes' },
            { href: '/dashboard', icon: '↩', label: 'User Area' },
          ].map((item) => (
            <a
              key={item.href}
              href={item.href}
              className="flex flex-col items-center gap-2 rounded-lg border border-gray-200 p-4 text-center hover:bg-gray-50"
            >
              <span className="text-2xl">{item.icon}</span>
              <span className="text-sm font-medium text-gray-700">{item.label}</span>
            </a>
          ))}
        </div>
      </Card>
    </div>
  )
}
