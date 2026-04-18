import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { adminApi } from '@/api/admin'
import { Input } from '@/components/ui/Input'
import { Table } from '@/components/ui/Table'
import { PageSpinner } from '@/components/ui/Spinner'
import { ticketStatusBadge } from '@/components/ui/Badge'
import { formatDateTime } from '@/utils/formatters'
import type { Ticket } from '@/api/types'

export function AdminTicketsPage() {
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState('')
  const [search, setSearch] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['admin-tickets', statusFilter],
    queryFn: () => adminApi.listTickets(statusFilter || undefined),
  })

  const tickets = (data?.tickets ?? []).filter((t) => {
    if (!search) return true
    return t.subject.toLowerCase().includes(search.toLowerCase()) || t.id.includes(search)
  })

  if (isLoading) return <PageSpinner />

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-slate-100">Тикеты поддержки</h1>

      <div className="flex flex-wrap gap-3">
        <Input
          placeholder="Поиск по теме или ID…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-xs"
        />
        <div className="flex gap-2">
          {['', 'open', 'answered', 'closed'].map((s) => (
            <button
              key={s}
              onClick={() => setStatusFilter(s)}
              className={`rounded-lg border px-3 py-2 text-sm font-medium transition-colors ${
                statusFilter === s
                  ? 'border-primary-500 bg-primary-500/10 text-primary-400'
                  : 'border-surface-600 text-slate-400 hover:bg-surface-700 hover:text-slate-300'
              }`}
            >
              {s === '' ? 'Все' : s === 'open' ? 'Открытые' : s === 'answered' ? 'Отвечено' : 'Закрытые'}
            </button>
          ))}
        </div>
      </div>

      <Table<Ticket>
        keyExtractor={(t) => t.id}
        data={tickets}
        emptyMessage="Тикеты не найдены"
        onRowClick={(t) => navigate(`/admin/tickets/${t.id}`)}
        columns={[
          {
            key: 'subject',
            header: 'Тема',
            render: (t) => <span className="font-medium">{t.subject}</span>,
          },
          {
            key: 'status',
            header: 'Статус',
            render: (t) => ticketStatusBadge(t.status),
          },
          {
            key: 'created_at',
            header: 'Создан',
            render: (t) => formatDateTime(t.created_at),
          },
          {
            key: 'updated_at',
            header: 'Последнее обновление',
            render: (t) => formatDateTime(t.updated_at),
          },
        ]}
      />
    </div>
  )
}
