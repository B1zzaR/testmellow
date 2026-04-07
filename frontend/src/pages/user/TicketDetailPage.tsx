import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ticketsApi } from '@/api/tickets'
import { Button } from '@/components/ui/Button'
import { Textarea } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { PageSpinner } from '@/components/ui/Spinner'
import { ticketStatusBadge } from '@/components/ui/Badge'
import { Icon } from '@/components/ui/Icons'
import { formatDateTime } from '@/utils/formatters'
import { useAuthStore } from '@/store/authStore'

export function TicketDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const currentUser = useAuthStore((s) => s.user)

  const { data, isLoading } = useQuery({
    queryKey: ['ticket', id],
    queryFn: () => ticketsApi.getById(id!),
    enabled: Boolean(id),
  })

  const [reply, setReply] = useState('')
  const [errorMsg, setErrorMsg] = useState('')

  const replyMutation = useMutation({
    mutationFn: (msg: string) => ticketsApi.reply(id!, { message: msg }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['ticket', id] })
      queryClient.invalidateQueries({ queryKey: ['tickets'] })
      setReply('')
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  if (isLoading) return <PageSpinner />
  if (!data) return <Alert variant="error" message="Тикет не найден" />

  const { ticket, messages } = data
  const isClosed = ticket.status === 'closed'

  return (
    <div className="mx-auto max-w-3xl space-y-4">
      {/* Header */}
      <div className="flex items-start gap-3">
        <button
          onClick={() => navigate('/tickets')}
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-gray-200 dark:border-surface-700 text-gray-500 dark:text-slate-500 hover:bg-gray-100 dark:hover:bg-surface-700 transition-colors"
        >
          <Icon name="back" size={16} />
        </button>
        <div className="flex-1 min-w-0">
          <h1 className="text-lg font-bold text-gray-900 dark:text-slate-100 truncate">{ticket.subject}</h1>
          <div className="mt-1 flex items-center gap-2">
            {ticketStatusBadge(ticket.status)}
            <span className="text-xs text-gray-400 dark:text-slate-600">
              Создан {formatDateTime(ticket.created_at)}
            </span>
          </div>
        </div>
      </div>

      {errorMsg && <Alert variant="error" message={errorMsg} />}
      {isClosed && <Alert variant="info" message="Этот тикет закрыт. Добавление ответов недоступно." />}

      {/* Chat messages */}
      <div className="rounded-xl border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-900 overflow-hidden">
        <div className="space-y-1 px-4 py-5">
          {messages.map((msg) => {
            const isOwn = msg.sender_id === currentUser?.id
            return (
              <div
                key={msg.id}
                className={`flex items-end gap-2 ${isOwn ? 'justify-end' : 'justify-start'}`}
              >
                {/* Avatar for support */}
                {!isOwn && (
                  <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-yellow-500/15 ring-1 ring-yellow-500/30 text-[10px] font-bold text-yellow-500">
                    S
                  </div>
                )}

                {/* Bubble */}
                <div
                  className={[
                    'max-w-[75%] rounded-2xl px-4 py-3 text-sm',
                    isOwn
                      ? 'rounded-br-sm bg-primary-500 text-white'
                      : 'rounded-bl-sm bg-gray-100 text-gray-800 dark:bg-surface-800 dark:text-slate-200',
                  ].join(' ')}
                >
                  <p className="leading-relaxed">{msg.body}</p>
                  <p
                    className={`mt-1.5 text-right text-[10px] ${
                      isOwn ? 'text-primary-200' : 'text-gray-400 dark:text-slate-600'
                    }`}
                  >
                    {formatDateTime(msg.created_at)}
                  </p>
                </div>

                {/* Avatar for user */}
                {isOwn && (
                  <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-surface-600 text-[10px] font-bold text-slate-300">
                    U
                  </div>
                )}
              </div>
            )
          })}
        </div>

        {/* Reply composer */}
        {!isClosed && (
          <div className="border-t border-gray-100 dark:border-surface-700 px-4 py-4">
            <Textarea
              placeholder="Введите ваш ответ…"
              rows={3}
              value={reply}
              onChange={(e) => setReply(e.target.value)}
            />
            <div className="mt-3 flex justify-end">
              <Button
                size="sm"
                loading={replyMutation.isPending}
                disabled={!reply.trim()}
                onClick={() => replyMutation.mutate(reply)}
              >
                <Icon name="external" size={12} />
                Отправить
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}


