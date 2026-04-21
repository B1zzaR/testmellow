import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Icon } from '@/components/ui/Icons'

export function AdminBroadcastPage() {
  const [message, setMessage] = useState('')
  const [result, setResult] = useState<{ queued: number; total: number } | null>(null)
  const [error, setError] = useState<string | null>(null)

  const broadcastMutation = useMutation({
    mutationFn: () => adminApi.broadcast({ message }),
    onSuccess: (data) => {
      setResult(data)
      setMessage('')
      setError(null)
    },
    onError: (e: Error) => {
      setError(e.message)
      setResult(null)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!message.trim()) return
    setResult(null)
    setError(null)
    broadcastMutation.mutate()
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-slate-100">Рассылка в Telegram</h1>

      <Card className="p-6 space-y-4">
        <div className="flex items-start gap-3 rounded-lg bg-yellow-500/10 border border-yellow-500/30 p-4">
          <Icon name="bell" size={18} className="mt-0.5 flex-shrink-0 text-yellow-400" />
          <div className="text-sm text-yellow-300">
            <p className="font-semibold mb-1">Важно</p>
            <p>
              Сообщение будет отправлено всем пользователям, привязавшим Telegram-аккаунт.
              Убедитесь, что текст корректен перед отправкой — отозвать сообщения невозможно.
            </p>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-2">
              Текст сообщения
              <span className="ml-2 text-xs text-slate-500">
                ({message.length} / 4096)
              </span>
            </label>
            <textarea
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              maxLength={4096}
              rows={8}
              placeholder="Введите текст рассылки... Поддерживается HTML (жирный, курсив, ссылки)."
              className="w-full rounded-lg border border-surface-600 bg-surface-800 px-3 py-2 text-sm text-slate-100 placeholder-slate-500 focus:border-yellow-500 focus:outline-none focus:ring-1 focus:ring-yellow-500 resize-y"
              required
            />
          </div>

          {error && (
            <div className="rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
              {error}
            </div>
          )}

          {result && (
            <div className="rounded-lg bg-green-500/10 border border-green-500/30 px-4 py-3 text-sm text-green-400">
              <Icon name="check-circle" size={16} className="inline mr-2" />
              Сообщений поставлено в очередь: <strong>{result.queued}</strong> из {result.total} пользователей с Telegram.
            </div>
          )}

          <div className="flex items-center justify-between pt-2">
            <p className="text-xs text-slate-500">
              Сообщения доставляются асинхронно через Telegram-бота
            </p>
            <Button
              type="submit"
              variant="primary"
              disabled={!message.trim() || broadcastMutation.isPending}
              loading={broadcastMutation.isPending}
            >
              <Icon name="megaphone" size={16} className="mr-2" />
              Отправить рассылку
            </Button>
          </div>
        </form>
      </Card>
    </div>
  )
}
