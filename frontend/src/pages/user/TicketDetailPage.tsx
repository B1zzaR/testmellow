import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ticketsApi } from '@/api/tickets'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Textarea } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { PageSpinner } from '@/components/ui/Spinner'
import { ticketStatusBadge } from '@/components/ui/Badge'
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
  if (!data) return <Alert variant="error" message="Ticket not found" />

  const { ticket, messages } = data
  const isClosed = ticket.status === 'closed'

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex items-center gap-3">
        <button
          onClick={() => navigate('/tickets')}
          className="rounded-lg p-2 text-gray-500 hover:bg-gray-100"
        >
          ←
        </button>
        <div>
          <h1 className="text-xl font-bold text-gray-900">{ticket.subject}</h1>
          <div className="mt-1 flex items-center gap-2">
            {ticketStatusBadge(ticket.status)}
            <span className="text-xs text-gray-400">
              Opened {formatDateTime(ticket.created_at)}
            </span>
          </div>
        </div>
      </div>

      {errorMsg && <Alert variant="error" message={errorMsg} />}
      {isClosed && <Alert variant="info" message="This ticket is closed. You cannot reply." />}

      <Card>
        <div className="space-y-4">
          {messages.map((msg) => (
            <div
              key={msg.id}
              className={`flex gap-3 ${msg.sender_id === currentUser?.id ? 'flex-row-reverse' : ''}`}
            >
              <div
                className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-bold ${
                  msg.is_admin
                    ? 'bg-primary-100 text-primary-700'
                    : 'bg-gray-200 text-gray-600'
                }`}
              >
                {msg.is_admin ? 'S' : 'U'}
              </div>
              <div
                className={`max-w-[75%] rounded-2xl px-4 py-3 text-sm ${
                  msg.sender_id === currentUser?.id
                    ? 'bg-primary-600 text-white'
                    : 'bg-gray-100 text-gray-800'
                }`}
              >
                <p>{msg.body}</p>
                <p
                  className={`mt-1 text-right text-xs ${
                    msg.sender_id === currentUser?.id ? 'text-primary-200' : 'text-gray-400'
                  }`}
                >
                  {formatDateTime(msg.created_at)}
                </p>
              </div>
            </div>
          ))}
        </div>

        {!isClosed && (
          <div className="mt-4 border-t border-gray-100 pt-4">
            <Textarea
              placeholder="Type your reply…"
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
                Send Reply
              </Button>
            </div>
          </div>
        )}
      </Card>
    </div>
  )
}
