import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { ticketsApi } from '@/api/tickets'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Input, Textarea } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { Modal } from '@/components/ui/Modal'
import { PageSpinner } from '@/components/ui/Spinner'
import { ticketStatusBadge } from '@/components/ui/Badge'
import { formatDateTime } from '@/utils/formatters'

export function TicketsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['tickets'],
    queryFn: ticketsApi.list,
  })

  const [modalOpen, setModalOpen] = useState(false)
  const [subject, setSubject] = useState('')
  const [message, setMessage] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const [fieldErrors, setFieldErrors] = useState<{ subject?: string; message?: string }>({})

  const createMutation = useMutation({
    mutationFn: ticketsApi.create,
    onSuccess: (ticket) => {
      queryClient.invalidateQueries({ queryKey: ['tickets'] })
      setModalOpen(false)
      setSubject('')
      setMessage('')
      navigate(`/tickets/${ticket.id}`)
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const handleCreate = () => {
    const errs: { subject?: string; message?: string } = {}
    if (subject.trim().length < 3) errs.subject = 'Subject must be at least 3 characters'
    if (message.trim().length < 1) errs.message = 'Message is required'
    if (Object.keys(errs).length > 0) { setFieldErrors(errs); return }
    setFieldErrors({})
    createMutation.mutate({ subject, message })
  }

  if (isLoading) return <PageSpinner />

  const tickets = data?.tickets ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Support Tickets</h1>
        <Button onClick={() => setModalOpen(true)} size="sm">
          + New Ticket
        </Button>
      </div>

      {errorMsg && <Alert variant="error" message={errorMsg} />}

      {tickets.length === 0 ? (
        <div className="rounded-xl border border-dashed border-gray-300 px-6 py-10 text-center">
          <p className="text-gray-400">No tickets yet. Open one if you need help.</p>
        </div>
      ) : (
        <Card>
          <div className="space-y-2">
            {tickets.map((ticket) => (
              <button
                key={ticket.id}
                onClick={() => navigate(`/tickets/${ticket.id}`)}
                className="w-full rounded-lg border border-gray-100 px-4 py-3 text-left hover:bg-gray-50"
              >
                <div className="flex items-center justify-between">
                  <p className="font-medium text-gray-800">{ticket.subject}</p>
                  {ticketStatusBadge(ticket.status)}
                </div>
                <p className="mt-1 text-xs text-gray-400">
                  Created {formatDateTime(ticket.created_at)} · Updated{' '}
                  {formatDateTime(ticket.updated_at)}
                </p>
              </button>
            ))}
          </div>
        </Card>
      )}

      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title="Open New Ticket"
        footer={
          <>
            <Button variant="secondary" onClick={() => setModalOpen(false)}>
              Cancel
            </Button>
            <Button loading={createMutation.isPending} onClick={handleCreate}>
              Submit
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Subject"
            placeholder="Brief description of your issue"
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            error={fieldErrors.subject}
          />
          <Textarea
            label="Message"
            placeholder="Describe your issue in detail"
            rows={4}
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            error={fieldErrors.message}
          />
        </div>
      </Modal>
    </div>
  )
}
