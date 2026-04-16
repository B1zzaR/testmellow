import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { QRCodeSVG } from 'qrcode.react'
import { profileApi, balanceApi } from '@/api/profile'
import { subscriptionsApi } from '@/api/subscriptions'
import { referralsApi } from '@/api/referrals'
import { publicApi } from '@/api/client'
import { StatCard, Card } from '@/components/ui/Card'
import { PageSpinner } from '@/components/ui/Spinner'
import { Button } from '@/components/ui/Button'
import { Icon } from '@/components/ui/Icons'
import { NotificationAlert } from '@/components/NotificationAlert'
import { PendingPayments } from '@/components/PendingPayments'
import { formatYAD, formatBytes, daysUntil, planLabel, formatDateTime } from '@/utils/formatters'

// ─── Activity event labels ────────────────────────────────────────────────────

const activityLabels: Record<string, string> = {
  login: 'Вход в аккаунт',
  register: 'Регистрация',
  password_change: 'Смена пароля',
  password_reset: 'Сброс пароля',
  telegram_link: 'Привязка Telegram',
  telegram_unlink: 'Отвязка Telegram',
  tfa_enable: 'Включение 2FA',
  tfa_disable: 'Отключение 2FA',
  subscription_buy: 'Покупка подписки',
  subscription_renew: 'Продление подписки',
  trial_activate: 'Активация пробного',
}

const activityIcons: Record<string, string> = {
  login: 'user',
  register: 'check-circle',
  password_change: 'lock',
  password_reset: 'lock',
  telegram_link: 'telegram',
  telegram_unlink: 'telegram',
  tfa_enable: 'shield',
  tfa_disable: 'shield',
  subscription_buy: 'tag',
  subscription_renew: 'refresh',
  trial_activate: 'zap',
}

// ─── Quick Connect ────────────────────────────────────────────────────────────

function QuickConnect() {
  const [copied, setCopied] = useState(false)
  const [showQR, setShowQR] = useState(false)
  const { data, isLoading } = useQuery({
    queryKey: ['connection'],
    queryFn: profileApi.getConnection,
    retry: 2,
    retryDelay: 3000,
  })

  if (isLoading) return null
  if (!data?.subscribe_url) return null

  const url = data.subscribe_url

  const copy = () => {
    navigator.clipboard.writeText(url).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <Card title="Быстрое подключение" glow>
      <div className="flex items-center gap-2 rounded-lg border border-gray-200 dark:border-surface-600 bg-gray-50 dark:bg-surface-800 px-3 py-2">
        <span className="flex-1 truncate font-mono text-xs text-gray-600 dark:text-slate-400">{url}</span>
        <button
          onClick={() => setShowQR((v) => !v)}
          title="QR-код"
          className={`flex shrink-0 items-center justify-center rounded-md border px-2 py-1 text-xs transition-all active:scale-95 ${
            showQR
              ? 'border-primary-500/50 bg-primary-500/10 text-primary-400'
              : 'border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-700 text-gray-500 dark:text-slate-400 hover:bg-gray-50 dark:hover:bg-surface-600'
          }`}
        >
          <Icon name="smartphone" size={14} />
        </button>
        <button
          onClick={copy}
          className="flex shrink-0 items-center gap-1.5 rounded-md border border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-700 px-2.5 py-1 text-xs font-medium text-gray-700 dark:text-slate-300 hover:bg-gray-50 dark:hover:bg-surface-600 active:scale-95 transition-all"
        >
          <Icon name={copied ? 'check' : 'copy'} size={12} className={copied ? 'text-primary-500' : ''} />
          <span className="hidden sm:inline whitespace-nowrap">{copied ? 'Скопировано' : 'Скопировать'}</span>
        </button>
      </div>

      {showQR && (
        <div className="mt-3 flex justify-center">
          <div className="rounded-xl border border-gray-200 dark:border-surface-600 bg-white dark:bg-surface-800 p-4">
            <QRCodeSVG
              value={url}
              size={180}
              bgColor="transparent"
              fgColor="currentColor"
              className="text-gray-900 dark:text-slate-100"
              level="M"
            />
            <p className="mt-2 text-center text-[10px] text-gray-400 dark:text-slate-600">Сканируйте QR в приложении</p>
          </div>
        </div>
      )}

      <p className="mt-2 text-xs text-gray-400 dark:text-slate-600">
        Happ, V2RayN, Hiddify, Streisand или любой совместимый клиент.
      </p>
    </Card>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export function DashboardPage() {
  const [dismissedNotifications, setDismissedNotifications] = useState<Set<string>>(new Set())

  const { data: profile, isLoading: profileLoading } = useQuery({
    queryKey: ['profile'],
    queryFn: profileApi.getProfile,
  })
  const { data: balance } = useQuery({
    queryKey: ['balance'],
    queryFn: balanceApi.getBalance,
  })
  const { data: subsData } = useQuery({
    queryKey: ['subscriptions'],
    queryFn: subscriptionsApi.list,
  })
  const { data: trafficData } = useQuery({
    queryKey: ['traffic'],
    queryFn: profileApi.getTraffic,
    retry: false,
    refetchInterval: 60_000,
  })
  const { data: notifications } = useQuery({
    queryKey: ['activeNotifications'],
    queryFn: publicApi.getActiveNotifications,
    refetchInterval: 30_000,
  })
  const { data: referralsData } = useQuery({
    queryKey: ['referrals'],
    queryFn: referralsApi.list,
  })
  const { data: activityData } = useQuery({
    queryKey: ['activity-recent'],
    queryFn: () => profileApi.getActivity(5),
  })

  if (profileLoading) return <PageSpinner />

  const activeSub = (subsData?.subscriptions ?? []).find((s) => s.status === 'active' || s.status === 'trial')
  const daysLeft = activeSub ? daysUntil(activeSub.expires_at) : 0
  const refCount = referralsData?.referral_count ?? 0
  const totalRefReward = (referralsData?.referrals ?? []).reduce((sum, r) => sum + r.total_reward, 0)

  // Security checklist
  const securityChecks = [
    { label: 'Пароль установлен', ok: true, icon: 'lock' as const },
    { label: 'Telegram привязан', ok: !!profile?.telegram_id, icon: 'telegram' as const },
    { label: '2FA включена', ok: !!profile?.tfa_enabled, icon: 'shield' as const },
  ]
  const securityScore = securityChecks.filter((c) => c.ok).length

  return (
    <div className="space-y-6">
      {/* Page heading */}
      <div>
        <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">
          {profile?.username ? `Привет, ${profile.username}` : 'Дашборд'}
        </h1>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">
          Ваша личная VPN-платформа
        </p>
      </div>

      {/* System notifications */}
      {notifications && notifications.length > 0 && (
        <div>
          {notifications
            .filter((n) => !dismissedNotifications.has(n.id))
            .map((notification) => (
              <NotificationAlert
                key={notification.id}
                notification={notification}
                onDismiss={() => {
                  setDismissedNotifications((prev) => new Set([...prev, notification.id]))
                }}
              />
            ))}
        </div>
      )}

      {/* Pending payments */}
      <PendingPayments />

      {/* Trial banner */}
      {!profile?.trial_used && (
        <div className="rounded-xl border border-primary-500/30 bg-primary-500/10 px-4 py-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-4">
          <div className="flex items-center gap-3 flex-1">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary-500/20 text-primary-500">
              <Icon name="tag" size={18} />
            </div>
            <div>
              <p className="text-sm font-semibold text-primary-400">У вас доступен бесплатный пробный период!</p>
              <p className="text-xs text-slate-400 mt-0.5">Активируйте 3 дня VPN-доступа прямо сейчас</p>
            </div>
          </div>
          <Link to="/subscriptions">
            <Button size="sm" className="w-full sm:w-auto">Активировать</Button>
          </Link>
        </div>
      )}

      {/* Stat cards — 4 key metrics */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard
          label="Баланс ЯД"
          value={formatYAD(balance?.yad_balance ?? profile?.yad_balance ?? 0)}
          icon={<Icon name="skull" size={28} />}
          accent
        />
        <StatCard
          label="Подписка"
          value={activeSub ? planLabel(activeSub.plan) : 'Нет'}
          sub={activeSub ? (daysLeft > 0 ? `${daysLeft} дн. осталось` : 'Истекла') : 'Не активна'}
          icon={<Icon name="shield" size={28} />}
        />
        <StatCard
          label="Рефералы"
          value={refCount}
          sub={totalRefReward > 0 ? `+${totalRefReward} ЯД заработано` : 'Пригласите друзей'}
          icon={<Icon name="users" size={28} />}
        />
        <StatCard
          label="Трафик"
          value={trafficData ? formatBytes(trafficData.used_bytes) : '—'}
          sub={trafficData ? (trafficData.limit_bytes > 0 ? `из ${formatBytes(trafficData.limit_bytes)}` : '∞ безлимит') : 'Нет данных'}
          icon={<Icon name="wifi" size={28} />}
        />
      </div>

      {/* Quick Connect */}
      {activeSub && daysLeft > 0 && <QuickConnect />}

      {/* No subscription prompt */}
      {!activeSub && (
        <div className="rounded-xl border border-dashed border-primary-900/40 bg-primary-500/5 p-6">
          <div className="flex items-start gap-4">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary-500/10 text-primary-500">
              <Icon name="shield" size={20} />
            </div>
            <div className="flex-1">
              <p className="font-semibold text-gray-900 dark:text-slate-100">Нет активной подписки</p>
              <p className="mt-1 text-sm text-gray-500 dark:text-slate-500">
                {profile?.trial_used
                  ? 'Пробный период исчерпан. Выберите тарифный план для продолжения.'
                  : 'У вас ещё нет активной подписки. Попробуйте бесплатно!'}
              </p>
              <div className="mt-4 flex flex-wrap gap-3">
                <Link to="/subscriptions">
                  <Button size="sm">Купить подписку</Button>
                </Link>
                {!profile?.trial_used && (
                  <Link to="/subscriptions">
                    <Button size="sm" variant="secondary">Активировать пробный</Button>
                  </Link>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Two-column: Security + Activity */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Account Security */}
        <Card
          title="Безопасность"
          action={
            <Link to="/settings">
              <Button size="sm" variant="secondary">Настроить</Button>
            </Link>
          }
        >
          <div className="mb-4 flex items-center gap-3">
            <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${
              securityScore === 3
                ? 'bg-primary-500/10 text-primary-500'
                : securityScore >= 2
                  ? 'bg-yellow-500/10 text-yellow-500'
                  : 'bg-red-500/10 text-red-500'
            }`}>
              <Icon name="shield" size={22} />
            </div>
            <div>
              <p className="text-sm font-semibold text-gray-900 dark:text-slate-100">
                {securityScore === 3 ? 'Максимальная защита' : securityScore >= 2 ? 'Хорошая защита' : 'Требуется внимание'}
              </p>
              <p className="text-xs text-gray-400 dark:text-slate-500">{securityScore} из {securityChecks.length} пунктов выполнено</p>
            </div>
          </div>
          <div className="space-y-2.5">
            {securityChecks.map((item) => (
              <div key={item.label} className="flex items-center gap-3 rounded-lg border border-gray-100 dark:border-surface-700 px-3 py-2.5">
                <div className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-md ${
                  item.ok ? 'bg-primary-500/10 text-primary-500' : 'bg-gray-100 dark:bg-surface-800 text-gray-400 dark:text-slate-600'
                }`}>
                  <Icon name={item.icon} size={14} />
                </div>
                <span className="flex-1 text-sm text-gray-700 dark:text-slate-300">{item.label}</span>
                <Icon
                  name={item.ok ? 'check-circle' : 'x-circle'}
                  size={16}
                  className={item.ok ? 'text-primary-500' : 'text-gray-300 dark:text-surface-600'}
                />
              </div>
            ))}
          </div>
        </Card>

        {/* Recent Activity */}
        <Card
          title="Последняя активность"
          action={
            <Link to="/settings">
              <Button size="sm" variant="secondary">Все события</Button>
            </Link>
          }
        >
          {activityData?.activity && activityData.activity.length > 0 ? (
            <div className="space-y-1">
              {activityData.activity.map((event) => (
                <div key={event.id} className="flex items-center gap-3 rounded-lg px-2 py-2.5 transition-colors hover:bg-gray-50 dark:hover:bg-surface-800">
                  <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-gray-100 dark:bg-surface-800 text-gray-500 dark:text-slate-500">
                    <Icon name={(activityIcons[event.event_type] ?? 'globe') as any} size={14} />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm text-gray-800 dark:text-slate-200">
                      {activityLabels[event.event_type] ?? event.event_type}
                    </p>
                    <p className="text-xs text-gray-400 dark:text-slate-600">{formatDateTime(event.created_at)}</p>
                  </div>
                  {event.ip && (
                    <span className="hidden shrink-0 text-xs text-gray-400 dark:text-slate-600 sm:block">{event.ip}</span>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <p className="py-4 text-center text-sm text-gray-400 dark:text-slate-600">Нет событий</p>
          )}
        </Card>
      </div>

      {/* Referral invite card */}
      <Card
        title="Приглашайте друзей"
        subtitle="Получайте 15% от каждого платежа приглашённых"
        action={
          <Link to="/referrals">
            <Button size="sm" variant="secondary">Подробнее</Button>
          </Link>
        }
      >
        <div className="flex items-center gap-3 rounded-lg border border-gray-200 dark:border-surface-600 bg-gray-50 dark:bg-surface-800 px-3 py-2.5">
          <Icon name="users" size={16} className="shrink-0 text-primary-500" />
          <span className="flex-1 font-mono text-sm text-gray-700 dark:text-slate-300">{referralsData?.referral_code ?? profile?.referral_code ?? '—'}</span>
          <CopyButton text={referralsData?.referral_code ?? profile?.referral_code ?? ''} />
        </div>
      </Card>
    </div>
  )
}

// ─── Tiny copy helper ─────────────────────────────────────────────────────────

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }
  return (
    <button
      onClick={copy}
      className="flex shrink-0 items-center gap-1.5 rounded-md border border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-700 px-2.5 py-1 text-xs font-medium text-gray-700 dark:text-slate-300 hover:bg-gray-50 dark:hover:bg-surface-600 active:scale-95 transition-all"
    >
      <Icon name={copied ? 'check' : 'copy'} size={12} className={copied ? 'text-primary-500' : ''} />
      <span className="hidden sm:inline">{copied ? 'Скопировано' : 'Скопировать'}</span>
    </button>
  )
}