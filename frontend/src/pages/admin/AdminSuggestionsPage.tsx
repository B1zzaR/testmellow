import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import type { Suggestion, SuggestionStatus } from '@/api/types'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Icon } from '@/components/ui/Icons'
import { PageSpinner } from '@/components/ui/Spinner'

const STATUS_LABELS: Record<SuggestionStatus, string> = {
  new: 'Новое',
  read: 'Прочитано',
  archived: 'В архиве',
}

const STATUS_COLORS: Record<SuggestionStatus, string> = {
  new: 'bg-blue-500/10 text-blue-400 border-blue-500/30',
  read: 'bg-green-500/10 text-green-400 border-green-500/30',
  archived: 'bg-slate-500/10 text-slate-400 border-slate-500/30',
}

const FILTERS: Array<{ value: string; label: string }> = [
  { value: '', label: 'Все' },
  { value: 'new', label: 'Новые' },
  { value: 'read', label: 'Прочитанные' },
  { value: 'archived', label: 'Архив' },
]

export function AdminSuggestionsPage() {
  const queryClient = useQueryClient()
  const [statusFilter, setStatusFilter] = useState('')
  const [expanded, setExpanded] = useState<string | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-suggestions', statusFilter],
    queryFn: () => adminApi.listSuggestions({ status: statusFilter || undefined, limit: 100 }),
  })

  const updateStatus = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      adminApi.updateSuggestionStatus(id, status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-suggestions'] })
    },
  })

  const handleMark = (s: Suggestion, status: SuggestionStatus) => {
    updateStatus.mutate({ id: s.id, status })
  }

  if (isLoading) return <PageSpinner />

  const suggestions = data?.suggestions ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-slate-100">Предложения от пользователей</h1>
        <span className="text-sm text-slate-400">{suggestions.length} записей</span>
      </div>

      {/* Filters */}
      <div className="flex gap-2 flex-wrap">
        {FILTERS.map((f) => (
          <button
            key={f.value}
            onClick={() => setStatusFilter(f.value)}
            className={[
              'rounded-full px-4 py-1.5 text-sm font-medium transition-colors',
              statusFilter === f.value
                ? 'bg-yellow-500 text-black'
                : 'bg-surface-800 text-slate-400 hover:bg-surface-700 hover:text-slate-200',
            ].join(' ')}
          >
            {f.label}
          </button>
        ))}
      </div>

      {suggestions.length === 0 ? (
        <Card className="p-12 text-center">
          <Icon name="lightbulb" size={40} className="mx-auto mb-3 text-slate-600" />
          <p className="text-slate-400">Предложений нет</p>
        </Card>
      ) : (
        <div className="space-y-3">
          {suggestions.map((s) => (
            <Card
              key={s.id}
              className={[
                'p-4 cursor-pointer transition-colors',
                s.status === 'new' ? 'border-blue-500/20' : '',
              ].join(' ')}
              onClick={() => setExpanded(expanded === s.id ? null : s.id)}
            >
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-2">
                    <span
                      className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium ${STATUS_COLORS[s.status]}`}
                    >
                      {STATUS_LABELS[s.status]}
                    </span>
                    <span className="text-xs text-slate-500">
                      {new Date(s.created_at).toLocaleString('ru-RU')}
                    </span>
                  </div>
                  <p
                    className={[
                      'text-sm text-slate-200 whitespace-pre-wrap',
                      expanded !== s.id ? 'line-clamp-2' : '',
                    ].join(' ')}
                  >
                    {s.body}
                  </p>
                </div>
                <Icon
                  name="chevron-down"
                  size={16}
                  className={`flex-shrink-0 mt-1 text-slate-500 transition-transform ${expanded === s.id ? 'rotate-180' : ''}`}
                />
              </div>

              {/* Actions (visible when expanded) */}
              {expanded === s.id && (
                <div
                  className="mt-4 flex gap-2 flex-wrap"
                  onClick={(e) => e.stopPropagation()}
                >
                  {s.status !== 'read' && (
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => handleMark(s, 'read')}
                      disabled={updateStatus.isPending}
                    >
                      <Icon name="check" size={14} className="mr-1" />
                      Отметить прочитанным
                    </Button>
                  )}
                  {s.status !== 'new' && (
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => handleMark(s, 'new')}
                      disabled={updateStatus.isPending}
                    >
                      Вернуть в новые
                    </Button>
                  )}
                  {s.status !== 'archived' && (
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-slate-400 hover:text-slate-200"
                      onClick={() => handleMark(s, 'archived')}
                      disabled={updateStatus.isPending}
                    >
                      <Icon name="x-circle" size={14} className="mr-1" />
                      В архив
                    </Button>
                  )}
                </div>
              )}
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
