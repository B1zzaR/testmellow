import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useRegister } from '@/hooks/useAuth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { SnakeLogo } from '@/components/ui/Icons'

export function RegisterPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [referralCode, setReferralCode] = useState('')
  const [validationError, setValidationError] = useState('')
  const register = useRegister()

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setValidationError('')

    if (password.length < 8) {
      setValidationError('Пароль должен содержать минимум 8 символов')
      return
    }
    if (password !== confirm) {
      setValidationError('Пароли не совпадают')
      return
    }

    register.mutate({
      username,
      password,
      referral_code: referralCode.trim() || undefined,
    })
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center bg-gray-50 px-4 dark:bg-[#07070d]">
      <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
        <div className="absolute left-1/2 top-0 h-[400px] w-[600px] -translate-x-1/2 rounded-full bg-primary-500/5 blur-3xl" />
      </div>

      <div className="relative w-full max-w-sm">
        <div className="mb-8 flex flex-col items-center text-center">
          <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl border border-primary-900/60 bg-primary-500/10">
            <SnakeLogo size={36} />
          </div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100">Создать аккаунт</h1>
          <p className="mt-1.5 text-sm text-gray-500 dark:text-slate-500">Присоединитесь к VPN-платформе</p>
        </div>

        <div className="rounded-2xl border border-gray-200 bg-white p-7 shadow-card dark:border-surface-600 dark:bg-surface-900">
          <form onSubmit={handleSubmit} className="space-y-4" noValidate>
            <Input
              label="Логин"
              placeholder="ваш_логин"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoComplete="username"
            />
            <Input
              label="Пароль"
              type="password"
              placeholder="Минимум 8 символов"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="new-password"
            />
            <Input
              label="Подтвердите пароль"
              type="password"
              placeholder="Повторите пароль"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
              autoComplete="new-password"
            />
            <Input
              label="Реферальный код (необязательно)"
              placeholder="напр. ABC123"
              value={referralCode}
              onChange={(e) => setReferralCode(e.target.value)}
              autoComplete="off"
              hint="Есть код от друга? Введите его здесь."
            />

            {(validationError || register.isError) && (
              <Alert
                variant="error"
                message={validationError || register.error?.message || 'Ошибка регистрации'}
              />
            )}

            <Button type="submit" className="w-full" loading={register.isPending} size="lg">
              Создать аккаунт
            </Button>
          </form>

          <p className="mt-6 text-center text-sm text-gray-500 dark:text-slate-500">
            Уже есть аккаунт?{' '}
            <Link to="/login" className="font-medium text-primary-500 hover:text-primary-400 transition-colors">
              Войти
            </Link>
          </p>
        </div>

        <p className="mt-6 text-center text-xs text-gray-400 dark:text-slate-600">
          Безопасно · Быстро · Надёжно
        </p>
      </div>
    </div>
  )
}
