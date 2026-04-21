import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { suggestionsApi } from '@/api/suggestions'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Icon } from '@/components/ui/Icons'

export function SuggestionsPage() {
  const [body, setBody] = useState('')
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')

  const mutation = useMutation({
    mutationFn: () => suggestionsApi.submit(body.trim()),
    onSuccess: (data) => {
      setSuccessMsg(data.message)
      setBody('')
      setErrorMsg('')
    },
    onError: (e: Error) => {
      setErrorMsg(e.message)
      setSuccessMsg('')
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!body.trim()) return
    mutation.mutate()
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100">
          Анонимная предложка
        </h1>
        <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
          Поделитесь идеей, пожеланием или замечанием. Ваша личность не раскрывается — мы не
          сохраняем информацию о том, кто оставил предложение.
        </p>
      </div>

      <Card className="p-6 space-y-5">
        {/* How it works */}
        <div className="flex gap-3 rounded-lg bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-700 p-4">
          <Icon name="lightbulb" size={18} className="mt-0.5 flex-shrink-0 text-blue-500 dark:text-blue-400" />
          <div className="text-sm text-blue-800 dark:text-blue-300 space-y-1">
            <p className="font-semibold">Как это работает</p>
            <ul className="list-disc list-inside space-y-0.5 text-blue-700 dark:text-blue-400">
              <li>Любое сообщение хранится без привязки к вашему аккаунту</li>
              <li>Администраторы видят только текст и дату отправки</li>
              <li>Лимит: 3 предложения в сутки</li>
            </ul>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">
              Ваше предложение
              <span className="ml-2 text-xs text-slate-400">
                ({body.length} / 3000)
              </span>
            </label>
            <textarea
              value={body}
              onChange={(e) => setBody(e.target.value)}
              maxLength={3000}
              rows={7}
              placeholder="Напишите ваши идеи, пожелания или замечания..."
              className="w-full rounded-lg border border-gray-200 dark:border-surface-600 bg-white dark:bg-surface-800 px-3 py-2 text-sm text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 resize-y"
              required
            />
          </div>

          {successMsg && (
            <div className="flex items-center gap-2 rounded-lg bg-green-50 dark:bg-green-950/30 border border-green-200 dark:border-green-700 px-4 py-3 text-sm text-green-700 dark:text-green-400">
              <Icon name="check-circle" size={16} className="flex-shrink-0" />
              {successMsg}
            </div>
          )}

          {errorMsg && (
            <div className="flex items-center gap-2 rounded-lg bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-700 px-4 py-3 text-sm text-red-700 dark:text-red-400">
              <Icon name="x-circle" size={16} className="flex-shrink-0" />
              {errorMsg}
            </div>
          )}

          <div className="flex justify-end pt-1">
            <Button
              type="submit"
              variant="primary"
              disabled={!body.trim() || mutation.isPending}
              loading={mutation.isPending}
            >
              <Icon name="lightbulb" size={16} className="mr-2" />
              Отправить анонимно
            </Button>
          </div>
        </form>
      </Card>

      <p className="text-center text-xs text-slate-400 dark:text-slate-600">
        Сервис предложений полностью анонимен. Не включайте личные данные в текст, если хотите остаться инкогнито.
      </p>
    </div>
  )
}
