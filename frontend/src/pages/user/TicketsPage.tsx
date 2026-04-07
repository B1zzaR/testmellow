import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { ticketsApi } from '@/api/tickets'
import { Button } from '@/components/ui/Button'
import { Input, Textarea } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { Modal } from '@/components/ui/Modal'
import { PageSpinner } from '@/components/ui/Spinner'
import { ticketStatusBadge } from '@/components/ui/Badge'
import { Icon } from '@/components/ui/Icons'
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
    if (subject.trim().length < 3) errs.subject = 'Тема должна содержать минимум 3 символа'
    if (message.trim().length < 1) errs.message = 'Сообщение обязательно'
    if (Object.keys(errs).length > 0) { setFieldErrors(errs); return }
    setFieldErrors({})
    createMutation.mutate({ subject, message })
  }

  if (isLoading) return <PageSpinner />

  const tickets = data?.tickets ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">Тикеты поддержки</h1>
          <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">Получите помощь от нашей команды</p>
        </div>
        <Button onClick={() => setModalOpen(true)} size="sm">
          <Icon name="message" size={14} />
          Новый тикет
        </Button>
      </div>

      {errorMsg && <Alert variant="error" message={errorMsg} />}

      {tickets.length === 0 ? (
        <div className="rounded-xl border border-dashed border-gray-300 dark:border-surface-600 py-12 text-center">
          <Icon name="message" size={32} className="mx-auto mb-3 text-gray-300 dark:text-slate-700" />
          <p className="text-sm font-medium text-gray-500 dark:text-slate-500">Тикетов пока нет</p>
          <p className="mt-1 text-xs text-gray-400 dark:text-slate-600">Создайте тикет, если нужна помощь</p>
        </div>
      ) : (
        <div className="space-y-2">
          {tickets.map((ticket) => (
            <button
              key={ticket.id}
              onClick={() => navigate(`/tickets/${ticket.id}`)}
              className="w-full rounded-xl border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-900 px-4 py-4 text-left transition-all hover:border-gray-300 dark:hover:border-surface-600 hover:shadow-sm dark:hover:bg-surface-800"
            >
              <div className="flex items-center justify-between gap-3">
                <p className="font-medium text-gray-800 dark:text-slate-200 truncate">{ticket.subject}</p>
                {ticketStatusBadge(ticket.status)}
              </div>
              <p className="mt-1.5 text-xs text-gray-400 dark:text-slate-600">
                Создан {formatDateTime(ticket.created_at)} · Обновлён {formatDateTime(ticket.updated_at)}
              </p>
            </button>
          ))}
        </div>
      )}

      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title="Создать тикет"
        footer={
          <>
            <Button variant="secondary" onClick={() => setModalOpen(false)}>Отмена</Button>
            <Button loading={createMutation.isPending} onClick={handleCreate}>Отправить</Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Тема"
            placeholder="Краткое описание проблемы"
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            error={fieldErrors.subject}
          />
          <Textarea
            label="Сообщение"
            placeholder="Опишите проблему подробнее…"
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
