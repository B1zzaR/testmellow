import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { adminApi } from '@/api/admin'
import { Table } from '@/components/ui/Table'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { PageSpinner } from '@/components/ui/Spinner'
import { formatDate, formatRubles, formatYAD } from '@/utils/formatters'
import type { User } from '@/api/types'

export function AdminUsersPage() {
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [page, setPage] = useState(1)
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Debounce search input — reset to page 1 on new query
  useEffect(() => {
    if (debounceTimer.current) clearTimeout(debounceTimer.current)
    debounceTimer.current = setTimeout(() => {
      setDebouncedSearch(search)
      setPage(1)
    }, 400)
    return () => { if (debounceTimer.current) clearTimeout(debounceTimer.current) }
  }, [search])

  const { data, isLoading } = useQuery({
    queryKey: ['admin-users', debouncedSearch, page],
    queryFn: () => adminApi.listUsers({ q: debouncedSearch || undefined, page }),
    staleTime: 30_000,
  })

  const users = data?.users ?? []
  const total = data?.total ?? 0
  const limit = data?.limit ?? 50
  const totalPages = Math.max(1, Math.ceil(total / limit))

  if (isLoading && page === 1 && !debouncedSearch) return <PageSpinner />

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-slate-100">Пользователи</h1>
        <span className="text-sm text-slate-500">{total} всего</span>
      </div>

      <Input
        placeholder="Поиск по логину или ID…"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="max-w-md"
      />

      <Table<User>
        keyExtractor={(u) => u.id}
        data={users}
        loading={isLoading}
        onRowClick={(u) => navigate(`/admin/users/${u.id}`)}
        emptyMessage="Пользователи не найдены"
        columns={[
          {
            key: 'username',
            header: 'Логин',
            render: (u) => u.username ?? '—',
          },
          {
            key: 'yad_balance',
            header: 'Баланс ЯД',
            render: (u) => formatYAD(u.yad_balance),
          },
          {
            key: 'ltv',
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

      {totalPages > 1 && (
        <div className="flex items-center justify-between pt-2">
          <Button
            variant="secondary"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
          >
            ← Назад
          </Button>
          <span className="text-sm text-slate-400">
            Страница {page} из {totalPages}
          </span>
          <Button
            variant="secondary"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            Вперёд →
          </Button>
        </div>
      )}
    </div>
  )
}
