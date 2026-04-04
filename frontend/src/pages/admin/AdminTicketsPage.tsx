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
      <h1 className="text-2xl font-bold text-gray-900">Support Tickets</h1>

      <div className="flex flex-wrap gap-3">
        <Input
          placeholder="Search by subject or ID…"
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
                  ? 'border-primary-500 bg-primary-50 text-primary-700'
                  : 'border-gray-200 text-gray-600 hover:bg-gray-50'
              }`}
            >
              {s === '' ? 'All' : s.charAt(0).toUpperCase() + s.slice(1)}
            </button>
          ))}
        </div>
      </div>

      <Table<Ticket>
        keyExtractor={(t) => t.id}
        data={tickets}
        emptyMessage="No tickets found"
        onRowClick={(t) => navigate(`/admin/tickets/${t.id}`)}
        columns={[
          {
            key: 'subject',
            header: 'Subject',
            render: (t) => <span className="font-medium">{t.subject}</span>,
          },
          {
            key: 'status',
            header: 'Status',
            render: (t) => ticketStatusBadge(t.status),
          },
          {
            key: 'created_at',
            header: 'Created',
            render: (t) => formatDateTime(t.created_at),
          },
          {
            key: 'updated_at',
            header: 'Last Update',
            render: (t) => formatDateTime(t.updated_at),
          },
        ]}
      />
    </div>
  )
}
