import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { adminApi } from '@/api/admin'
import { Table } from '@/components/ui/Table'
import { Input } from '@/components/ui/Input'
import { Badge } from '@/components/ui/Badge'
import { PageSpinner } from '@/components/ui/Spinner'
import { formatDate, formatRubles, formatYAD } from '@/utils/formatters'
import type { User } from '@/api/types'

export function AdminUsersPage() {
  const navigate = useNavigate()
  const { data, isLoading } = useQuery({
    queryKey: ['admin-users'],
    queryFn: adminApi.listUsers,
  })
  const [search, setSearch] = useState('')

  const users = (data?.users ?? []).filter((u) => {
    if (!search) return true
    const q = search.toLowerCase()
    return (
      u.email?.toLowerCase().includes(q) ||
      u.username?.toLowerCase().includes(q) ||
      u.id.includes(q)
    )
  })

  if (isLoading) return <PageSpinner />

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-slate-100">Пользователи</h1>
        <span className="text-sm text-slate-500">{(data?.users ?? []).length} total</span>
      </div>

      <Input
        placeholder="Поиск по email, логину или ID…"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="max-w-md"
      />

      <Table<User>
        keyExtractor={(u) => u.id}
        data={users}
        loading={false}
        onRowClick={(u) => navigate(`/admin/users/${u.id}`)}
        emptyMessage="Пользователи не найдены"
        columns={[
          {
            key: 'email',
            header: 'Email / Логин',
            render: (u) => (
              <div>
                <p className="font-medium">{u.email ?? '—'}</p>
                {u.username && <p className="text-xs text-gray-400">{u.username}</p>}
              </div>
            ),
          },
          {
            key: 'yad_balance',
            header: 'Баланс ЯД',
            render: (u) => formatYAD(u.yad_balance),
          },
          {
            key: 'ltv_kopecks',
            header: 'LTV',
            render: (u) => formatRubles(u.ltv_kopecks),
          },
          {
            key: 'risk_score',
            header: 'Риск',
            render: (u) => (
              <span
                className={`font-semibold ${
                  u.risk_score >= 70
                    ? 'text-red-600'
                    : u.risk_score >= 40
                    ? 'text-yellow-600'
                    : 'text-green-600'
                }`}
              >
                {u.risk_score}
              </span>
            ),
          },
          {
            key: 'status',
            header: 'Статус',
            render: (u) => (
              <div className="flex flex-wrap gap-1">
                {u.is_admin && <Badge label="Админ" variant="purple" />}
                {u.is_banned && <Badge label="Заблокирован" variant="red" />}
                {!u.is_banned && !u.is_admin && <Badge label="Активен" variant="green" />}
              </div>
            ),
          },
          {
            key: 'created_at',
            header: 'Зарегистрирован',
            render: (u) => formatDate(u.created_at),
          },
        ]}
      />
    </div>
  )
}
