import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useLogin } from '@/hooks/useAuth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { SnakeLogo } from '@/components/ui/Icons'

export function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [validationError, setValidationError] = useState('')
  const login = useLogin()

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setValidationError('')

    const trimmed = username.trim()
    if (trimmed.length < 3) {
      setValidationError('Логин должен содержать минимум 3 символа')
      return
    }
    if (password.length < 8) {
      setValidationError('Пароль должен содержать минимум 8 символов')
      return
    }

    login.mutate({ username: trimmed, password })
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center bg-gray-50 px-4 dark:bg-[#07070d]">
      {/* Subtle radial glow */}
      <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
        <div className="absolute left-1/2 top-0 h-[400px] w-[600px] -translate-x-1/2 rounded-full bg-primary-500/5 blur-3xl" />
      </div>

      <div className="relative w-full max-w-sm">
        {/* Brand block */}
        <div className="mb-8 flex flex-col items-center text-center">
          <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl border border-primary-900/60 bg-primary-500/10">
            <SnakeLogo size={36} />
          </div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100">Добро пожаловать</h1>
          <p className="mt-1.5 text-sm text-gray-500 dark:text-slate-500">Войдите в свой VPN-аккаунт</p>
        </div>

        {/* Card */}
        <div className="rounded-2xl border border-gray-200 bg-white p-7 shadow-card dark:border-surface-600 dark:bg-surface-900 dark:shadow-card">
          <form onSubmit={handleSubmit} className="space-y-5" noValidate>
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
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="current-password"
            />

            {(validationError || login.isError) && (
              <Alert variant="error" message={validationError || (login.error?.message ?? 'Ошибка входа')} />
            )}

            <Button type="submit" className="w-full" loading={login.isPending} size="lg">
              Войти
            </Button>
          </form>

          <p className="mt-6 text-center text-sm text-gray-500 dark:text-slate-500">
            Нет аккаунта?{' '}
            <Link to="/register" className="font-medium text-primary-500 hover:text-primary-400 transition-colors">
              Зарегистрироваться
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
