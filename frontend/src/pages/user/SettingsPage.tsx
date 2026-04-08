import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { profileApi } from '@/api/profile'
import { useProfile } from '@/hooks/useProfile'
import { Card } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { Icon } from '@/components/ui/Icons'

// ─── Telegram section ──────────────────────────────────────────────────────────

function TelegramSection() {
  const queryClient = useQueryClient()
  const { data: profile } = useProfile()
  const [flash, setFlash] = useState<{ type: 'success' | 'error'; msg: string } | null>(null)
  const [linkCode, setLinkCode] = useState<{ code: string; bot_username: string } | null>(null)
  const [copied, setCopied] = useState(false)

  const unlinkMutation = useMutation({
    mutationFn: () => profileApi.setTelegramID(null),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      setFlash({ type: 'success', msg: 'Telegram отвязан' })
    },
    onError: (e: Error) => setFlash({ type: 'error', msg: e.message }),
  })

  const linkCodeMutation = useMutation({
    mutationFn: () => profileApi.requestLinkCode(),
    onSuccess: (data) => {
      setLinkCode(data)
      setFlash(null)
    },
    onError: (e: Error) => setFlash({ type: 'error', msg: e.message }),
  })

  const copyCode = () => {
    if (!linkCode) return
    const cmd = `/link ${linkCode.code}`
    navigator.clipboard.writeText(cmd).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const isLinked = profile?.telegram_id != null

  return (
    <div className="space-y-4">
      <div>
        <h2 className="flex items-center gap-2 text-base font-semibold text-gray-900 dark:text-slate-100">
          <Icon name="telegram" size={18} className="text-primary-500" />
          Telegram уведомления
        </h2>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">
          Привяжите аккаунт, чтобы получать уведомления о подписке прямо в Telegram
        </p>
      </div>

      {flash && <Alert variant={flash.type} message={flash.msg} />}

      {isLinked ? (
        <div className="space-y-3">
          <div className="flex items-center gap-3 rounded-lg border border-green-200 bg-green-50 px-4 py-3 dark:border-green-800/40 dark:bg-green-500/10">
            <Icon name="check-circle" size={18} className="shrink-0 text-green-500" />
            <div>
              <p className="text-sm font-medium text-green-700 dark:text-green-400">
                Telegram привязан
              </p>
              <p className="text-xs text-green-600 dark:text-green-500">
                ID: {profile.telegram_id}
              </p>
            </div>
          </div>
          <Button
            variant="secondary"
            onClick={() => { setFlash(null); unlinkMutation.mutate() }}
            loading={unlinkMutation.isPending}
          >
            Отвязать Telegram
          </Button>
        </div>
      ) : linkCode ? (
        <div className="space-y-3">
          <div className="rounded-xl border border-primary-900/40 bg-primary-500/5 p-4 space-y-3">
            <div className="flex items-center gap-2">
              <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-500 text-xs font-bold text-white">1</div>
              <p className="text-sm text-gray-700 dark:text-slate-300">
                Откройте{' '}
                {linkCode.bot_username ? (
                  <a
                    href={`https://t.me/${linkCode.bot_username}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="font-semibold text-primary-500 underline underline-offset-2"
                  >
                    @{linkCode.bot_username}
                  </a>
                ) : (
                  <span className="font-semibold">нашего бота</span>
                )}{' '}
                в Telegram
              </p>
            </div>
            <div className="flex items-center gap-2">
              <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-primary-500 text-xs font-bold text-white">2</div>
              <p className="text-sm text-gray-700 dark:text-slate-300">Отправьте боту команду:</p>
            </div>
            <div className="flex items-center gap-2 rounded-lg border border-gray-200 dark:border-surface-600 bg-white dark:bg-surface-800 px-3 py-2">
              <span className="flex-1 font-mono text-sm font-medium text-gray-900 dark:text-slate-100">
                /link {linkCode.code}
              </span>
              <button
                onClick={copyCode}
                className="flex shrink-0 items-center gap-1.5 rounded-md border border-gray-300 dark:border-surface-600 bg-gray-50 dark:bg-surface-700 px-3 py-1 text-xs font-medium text-gray-700 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-surface-600 active:scale-95 transition-all"
              >
                <Icon name={copied ? 'check' : 'copy'} size={12} className={copied ? 'text-primary-500' : ''} />
                {copied ? 'Скопировано' : 'Скопировать'}
              </button>
            </div>
            <p className="text-xs text-gray-400 dark:text-slate-600">
              Код действителен 5 минут. После отправки страницу можно обновить.
            </p>
          </div>
          <Button
            variant="secondary"
            onClick={() => setLinkCode(null)}
          >
            Отмена
          </Button>
        </div>
      ) : (
        <div className="space-y-3">
          <Button
            onClick={() => linkCodeMutation.mutate()}
            loading={linkCodeMutation.isPending}
          >
            <Icon name="telegram" size={16} />
            Привязать через бота
          </Button>
          <p className="text-xs text-gray-400 dark:text-slate-600">
            Вам будет сгенерирован код — отправьте его боту{' '}
            <a
              href="https://t.me/mellowpn_bot"
              target="_blank"
              rel="noopener noreferrer"
              className="font-medium text-primary-500 hover:underline"
            >
              @mellowpn_bot
            </a>
            , и аккаунты свяжутся автоматически.
          </p>
        </div>
      )}
    </div>
  )
}

// ─── Password section ─────────────────────────────────────────────────────────

function PasswordSection() {
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
    <div className="space-y-4">
      <div>
        <h2 className="flex items-center gap-2 text-base font-semibold text-gray-900 dark:text-slate-100">
          <Icon name="lock" size={18} className="text-primary-500" />
          Безопасность
        </h2>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">
          Обновите пароль для входа в аккаунт
        </p>
      </div>

      {flash && <Alert variant={flash.type} message={flash.msg} />}

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
        <Button type="submit" loading={mutation.isPending}>
          Изменить пароль
        </Button>
      </form>
    </div>
  )
}

// ─── Page ──────────────────────────────────────────────────────────────────────

export function SettingsPage() {
  return (
    <div className="mx-auto max-w-3xl space-y-6 py-6">
      <div>
        <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">Настройки</h1>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">Управление аккаунтом и уведомлениями</p>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <TelegramSection />
        </Card>
        <Card>
          <PasswordSection />
        </Card>
      </div>
    </div>
  )
}
