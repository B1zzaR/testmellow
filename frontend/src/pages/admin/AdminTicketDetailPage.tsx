import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import { Button } from '@/components/ui/Button'
import { Textarea } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { PageSpinner } from '@/components/ui/Spinner'
import { ticketStatusBadge } from '@/components/ui/Badge'
import { Icon } from '@/components/ui/Icons'
import { formatDateTime } from '@/utils/formatters'
export function AdminTicketDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['admin-ticket', id],
    queryFn: () => adminApi.getTicket(id!),
    enabled: Boolean(id),
  })

  const [reply, setReply] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const [successMsg, setSuccessMsg] = useState('')

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['admin-ticket', id] })
    queryClient.invalidateQueries({ queryKey: ['admin-tickets'] })
  }

  const replyMutation = useMutation({
    mutationFn: (msg: string) => adminApi.replyToTicket(id!, { message: msg }),
    onSuccess: () => {
      setReply('')
      setSuccessMsg('Ответ отправлен')
      invalidate()
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const closeMutation = useMutation({
    mutationFn: () => adminApi.closeTicket(id!),
    onSuccess: () => {
      setSuccessMsg('Тикет закрыт')
      invalidate()
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  if (isLoading) return <PageSpinner />
  if (!data) return <Alert variant="error" message="Тикет не найден" />

  const { ticket, messages } = data
  const isClosed = ticket.status === 'closed'

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex items-center gap-3">
        <button
          onClick={() => navigate('/admin/tickets')}
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-surface-700 text-slate-500 hover:bg-surface-700 transition-colors"
        >
          <Icon name="back" size={16} />
        </button>
        <div className="flex-1">
          <h1 className="text-xl font-bold text-slate-100">{ticket.subject}</h1>
          <div className="mt-1 flex items-center gap-2">
            {ticketStatusBadge(ticket.status)}
            <span className="text-xs text-slate-600">
              User: {ticket.user_id.slice(0, 8)}… · {formatDateTime(ticket.created_at)}
            </span>
          </div>
        </div>
        {!isClosed && (
          <Button
            variant="secondary"
            size="sm"
            loading={closeMutation.isPending}
            onClick={() => closeMutation.mutate()}
          >
            Закрыть тикет
          </Button>
        )}
      </div>

      {successMsg && <Alert variant="success" message={successMsg} />}
      {errorMsg && <Alert variant="error" message={errorMsg} />}

      <div className="rounded-xl border border-surface-700 bg-surface-900 overflow-hidden">
        <div className="space-y-1 px-4 py-5">
          {messages.map((msg) => (
            <div
              key={msg.id}
              className={`flex items-end gap-2 ${msg.is_admin ? 'justify-end' : 'justify-start'}`}
            >
              {/* User avatar */}
              {!msg.is_admin && (
                <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-surface-600 ring-1 ring-surface-500 text-[10px] font-bold text-slate-300">
                  U
                </div>
              )}

              {/* Bubble */}
              <div
                className={[
                  'max-w-[75%] rounded-2xl px-4 py-3 text-sm',
                  msg.is_admin
                    ? 'rounded-br-sm bg-yellow-500/20 text-yellow-100 ring-1 ring-yellow-500/30'
                    : 'rounded-bl-sm bg-surface-800 text-slate-200',
                ].join(' ')}
              >
                <p className="leading-relaxed">{msg.body}</p>
                <p
                  className={`mt-1.5 text-right text-[10px] ${
                    msg.is_admin ? 'text-yellow-600' : 'text-slate-600'
                  }`}
                >
                  {msg.is_admin ? 'Админ' : 'Пользователь'} · {formatDateTime(msg.created_at)}
                </p>
              </div>

              {/* Admin avatar */}
              {msg.is_admin && (
                <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-yellow-500/15 ring-1 ring-yellow-500/30 text-[10px] font-bold text-yellow-500">
                  A
                </div>
              )}
            </div>
          ))}
        </div>

        {!isClosed && (
          <div className="border-t border-surface-700 px-4 py-4">
            <Textarea
              placeholder="Введите ответ администратора…"
              rows={3}
              value={reply}
              onChange={(e) => setReply(e.target.value)}
            />
            <div className="mt-3 flex justify-between">
              <span />
              <Button
                size="sm"
                loading={replyMutation.isPending}
                disabled={!reply.trim()}
                onClick={() => replyMutation.mutate(reply)}
              >
                Отправить
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
