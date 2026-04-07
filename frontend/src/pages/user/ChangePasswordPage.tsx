import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { profileApi } from '@/api/profile'
import { Card } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'

export function ChangePasswordPage() {
  const navigate = useNavigate()
  const [oldPw, setOldPw] = useState('')
  const [newPw, setNewPw] = useState('')
  const [confirmPw, setConfirmPw] = useState('')
  const [flash, setFlash] = useState<{ type: 'success' | 'error'; msg: string } | null>(null)

  const mutation = useMutation({
    mutationFn: () => profileApi.changePassword(oldPw, newPw),
    onSuccess: () => {
      setFlash({ type: 'success', msg: 'Пароль успешно изменён' })
      setOldPw('')
      setNewPw('')
      setConfirmPw('')
      setTimeout(() => navigate('/dashboard'), 1500)
    },
    onError: (e: Error) => setFlash({ type: 'error', msg: e.message }),
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (newPw !== confirmPw) {
      setFlash({ type: 'error', msg: 'Новые пароли не совпадают' })
      return
    }
    if (newPw.length < 8) {
      setFlash({ type: 'error', msg: 'Новый пароль должен быть не менее 8 символов' })
      return
    }
    setFlash(null)
    mutation.mutate()
  }

  return (
    <div className="mx-auto max-w-md space-y-5 py-6">
      <div>
        <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">Смена пароля</h1>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">Обновите пароль для входа в аккаунт</p>
      </div>

      {flash && <Alert variant={flash.type} message={flash.msg} />}

      <Card>
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            label="Текущий пароль"
            type="password"
            value={oldPw}
            onChange={(e) => setOldPw(e.target.value)}
            autoComplete="current-password"
            required
          />
          <Input
            label="Новый пароль"
            type="password"
            value={newPw}
            onChange={(e) => setNewPw(e.target.value)}
            autoComplete="new-password"
            required
            minLength={8}
          />
          <Input
            label="Повторите новый пароль"
            type="password"
            value={confirmPw}
            onChange={(e) => setConfirmPw(e.target.value)}
            autoComplete="new-password"
            required
          />
          <div className="flex gap-3 pt-2">
            <Button type="submit" loading={mutation.isPending}>
              Изменить пароль
            </Button>
            <Button type="button" variant="secondary" onClick={() => navigate(-1)}>
              Отмена
            </Button>
          </div>
        </form>
      </Card>
    </div>
  )
}
