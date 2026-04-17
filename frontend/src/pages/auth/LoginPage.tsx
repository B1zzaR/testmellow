import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useLogin } from '@/hooks/useAuth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { SnakeLogo, Icon } from '@/components/ui/Icons'

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

  // 2FA waiting screen
  if (login.tfaState.required) {
    return (
      <div className="relative flex min-h-screen items-center justify-center bg-gradient-to-b from-gray-50 to-white px-4 dark:from-surface-950 dark:to-surface-950 dark:bg-surface-950">
        <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
          <div className="absolute left-1/2 top-0 h-[400px] w-[600px] -translate-x-1/2 rounded-full bg-primary-500/5 blur-3xl" />
        </div>

        <div className="relative w-full max-w-sm">
          <div className="mb-8 flex flex-col items-center text-center">
            <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl border border-primary-900/60 bg-primary-500/10">
              <SnakeLogo size={36} />
            </div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-slate-100">Подтверждение входа</h1>
            <p className="mt-1.5 text-sm text-gray-500 dark:text-slate-500">Подтвердите вход через Telegram</p>
          </div>

          <div className="rounded-2xl border border-gray-200 bg-white p-7 shadow-card dark:border-surface-600 dark:bg-surface-900 dark:shadow-card">
            {login.tfaState.status === 'denied' ? (
              <div className="space-y-4 text-center">
                <div className="flex justify-center">
                  <div className="flex h-14 w-14 items-center justify-center rounded-full bg-red-100 dark:bg-red-500/10">
                    <Icon name="x-circle" size={28} className="text-red-500" />
                  </div>
                </div>
                <p className="text-sm font-medium text-red-600 dark:text-red-400">Вход отклонён в Telegram</p>
                <p className="text-xs text-gray-500 dark:text-slate-500">
                  Если это были вы, попробуйте войти ещё раз.
                </p>
                <Button onClick={login.resetTFA} variant="secondary" className="w-full">
                  Попробовать снова
                </Button>
              </div>
            ) : login.tfaState.status === 'expired' ? (
              <div className="space-y-4 text-center">
                <div className="flex justify-center">
                  <div className="flex h-14 w-14 items-center justify-center rounded-full bg-amber-100 dark:bg-amber-500/10">
                    <Icon name="clock" size={28} className="text-amber-500" />
                  </div>
                </div>
                <p className="text-sm font-medium text-amber-600 dark:text-amber-400">Время подтверждения истекло</p>
                <p className="text-xs text-gray-500 dark:text-slate-500">
                  Попробуйте войти ещё раз.
                </p>
                <Button onClick={login.resetTFA} variant="secondary" className="w-full">
                  Попробовать снова
                </Button>
              </div>
            ) : (
              <div className="space-y-4 text-center">
                <div className="flex justify-center">
                  <div className="flex h-14 w-14 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-500/10">
                    <Icon name="telegram" size={28} className="text-primary-500" />
                  </div>
                </div>
                <p className="text-sm font-medium text-gray-900 dark:text-slate-100">
                  Откройте Telegram и подтвердите вход
                </p>
                <p className="text-xs text-gray-500 dark:text-slate-500">
                  Мы отправили запрос в вашего Telegram-бота. Нажмите «Подтвердить» для входа.
                </p>
                <div className="flex items-center justify-center gap-2 text-sm text-gray-400 dark:text-slate-600">
                  <div className="h-4 w-4 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" />
                  Ожидание подтверждения…
                </div>
                <Button onClick={login.resetTFA} variant="secondary" className="w-full" size="sm">
                  Отмена
                </Button>
              </div>
            )}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center bg-gradient-to-b from-gray-50 to-white px-4 dark:from-surface-950 dark:to-surface-950 dark:bg-surface-950">
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
          <p className="mt-3 text-center text-sm text-gray-500 dark:text-slate-500">
            Забыли пароль?{' '}
            <a
              href="https://t.me/mellowpn_bot"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium text-primary-500 hover:text-primary-400 transition-colors"
            >
              Восстановите через Telegram-бота
            </a>
          </p>

          <div className="mt-5 rounded-xl border border-primary-500/20 bg-primary-500/5 p-3.5 text-center">
            <p className="text-xs text-gray-500 dark:text-slate-400">
              Управляйте подпиской через{' '}
              <a
                href="https://t.me/mellowpn_bot"
                target="_blank"
                rel="noopener noreferrer"
                className="font-medium text-primary-500 hover:text-primary-400 transition-colors"
              >
                Telegram-бота @mellowpn_bot
              </a>
            </p>
          </div>
        </div>

        <p className="mt-6 text-center text-xs text-gray-400 dark:text-slate-600">
          Безопасно · Быстро · Надёжно
        </p>
      </div>
    </div>
  )
}
