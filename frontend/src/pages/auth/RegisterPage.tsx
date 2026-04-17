import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useRegister } from '@/hooks/useAuth'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { SnakeLogo } from '@/components/ui/Icons'

function passwordStrength(pwd: string): { score: number; label: string; color: string } {
  if (pwd.length === 0) return { score: 0, label: '', color: '' }
  let score = 0
  if (pwd.length >= 8) score++
  if (pwd.length >= 12) score++
  if (/[A-Z]/.test(pwd)) score++
  if (/[0-9]/.test(pwd)) score++
  if (/[^a-zA-Z0-9]/.test(pwd)) score++
  if (score <= 1) return { score, label: 'Слабый', color: 'bg-red-500' }
  if (score <= 2) return { score, label: 'Средний', color: 'bg-yellow-500' }
  if (score <= 3) return { score, label: 'Хороший', color: 'bg-blue-500' }
  return { score, label: 'Надёжный', color: 'bg-primary-500' }
}

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

    const trimmedUsername = username.trim()
    if (trimmedUsername.length < 3) {
      setValidationError('Логин должен содержать минимум 3 символа')
      return
    }
    if (trimmedUsername.length > 32) {
      setValidationError('Логин должен быть не длиннее 32 символов')
      return
    }
    if (!/^[a-zA-Z0-9_]+$/.test(trimmedUsername)) {
      setValidationError('Логин может содержать только латинские буквы, цифры и "_"')
      return
    }

    register.mutate({
      username: trimmedUsername,
      password,
      referral_code: referralCode.trim() || undefined,
    })
  }

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
            {password.length > 0 && (() => {
              const strength = passwordStrength(password)
              return (
                <div className="-mt-2">
                  <div className="flex gap-1">
                    {[1, 2, 3, 4].map((i) => (
                      <div
                        key={i}
                        className={[
                          'h-1 flex-1 rounded-full transition-colors',
                          strength.score >= i ? strength.color : 'bg-gray-200 dark:bg-surface-600',
                        ].join(' ')}
                      />
                    ))}
                  </div>
                  <p className={`mt-1 text-xs font-medium ${
                    strength.score <= 1 ? 'text-red-500' :
                    strength.score <= 2 ? 'text-yellow-500' :
                    strength.score <= 3 ? 'text-blue-500' : 'text-primary-500'
                  }`}>{strength.label}</p>
                </div>
              )
            })()}
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

          <div className="mt-5 rounded-xl border border-primary-500/20 bg-primary-500/5 p-3.5 text-center">
            <p className="text-xs text-gray-500 dark:text-slate-400">
              Также доступно через{' '}
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
