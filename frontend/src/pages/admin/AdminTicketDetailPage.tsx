import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Textarea } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { PageSpinner } from '@/components/ui/Spinner'
import { ticketStatusBadge } from '@/components/ui/Badge'
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
      setSuccessMsg('Reply sent')
      invalidate()
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const closeMutation = useMutation({
    mutationFn: () => adminApi.closeTicket(id!),
    onSuccess: () => {
      setSuccessMsg('Ticket closed')
      invalidate()
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
          onClick={() => navigate('/admin/tickets')}
          className="rounded-lg p-2 text-gray-500 hover:bg-gray-100"
        >
          ←
        </button>
        <div className="flex-1">
          <h1 className="text-xl font-bold text-gray-900">{ticket.subject}</h1>
          <div className="mt-1 flex items-center gap-2">
            {ticketStatusBadge(ticket.status)}
            <span className="text-xs text-gray-400">
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
            Close Ticket
          </Button>
        )}
      </div>

      {successMsg && <Alert variant="success" message={successMsg} />}
      {errorMsg && <Alert variant="error" message={errorMsg} />}

      <Card>
        <div className="space-y-4">
          {messages.map((msg) => (
            <div
              key={msg.id}
              className={`flex gap-3 ${msg.is_admin ? 'flex-row-reverse' : ''}`}
            >
              <div
                className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-bold ${
                  msg.is_admin
                    ? 'bg-primary-100 text-primary-700'
                    : 'bg-gray-200 text-gray-600'
                }`}
              >
                {msg.is_admin ? 'A' : 'U'}
              </div>
              <div
                className={`max-w-[75%] rounded-2xl px-4 py-3 text-sm ${
                  msg.is_admin
                    ? 'bg-primary-600 text-white'
                    : 'bg-gray-100 text-gray-800'
                }`}
              >
                <p>{msg.body}</p>
                <p
                  className={`mt-1 text-right text-xs ${
                    msg.is_admin ? 'text-primary-200' : 'text-gray-400'
                  }`}
                >
                  {msg.is_admin ? 'Admin' : 'User'} · {formatDateTime(msg.created_at)}
                </p>
              </div>
            </div>
          ))}
        </div>

        {!isClosed && (
          <div className="mt-4 border-t border-gray-100 pt-4">
            <Textarea
              placeholder="Type admin reply…"
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
