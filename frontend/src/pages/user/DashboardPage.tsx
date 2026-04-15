import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { QRCodeSVG } from 'qrcode.react'
import { profileApi, balanceApi } from '@/api/profile'
import { subscriptionsApi } from '@/api/subscriptions'
import { devicesApi } from '@/api/devices'
import { publicApi } from '@/api/client'
import { StatCard, Card } from '@/components/ui/Card'
import { PageSpinner } from '@/components/ui/Spinner'
import { Button } from '@/components/ui/Button'
import { subscriptionStatusBadge } from '@/components/ui/Badge'
import { Icon } from '@/components/ui/Icons'
import { NotificationAlert } from '@/components/NotificationAlert'
import { PendingPayments } from '@/components/PendingPayments'
import { DeviceList } from '@/components/DeviceList'
import { formatDate, formatYAD, daysUntil, planLabel } from '@/utils/formatters'

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 Б'
  const units = ['Б', 'КБ', 'МБ', 'ГБ', 'ТБ']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

// ─── Connection link row ──────────────────────────────────────────────────────

function ConnectionRow() {
  const [copied, setCopied] = useState(false)
  const [showQR, setShowQR] = useState(false)
  const { data, isLoading } = useQuery({
    queryKey: ['connection'],
    queryFn: profileApi.getConnection,
    retry: 2,
    retryDelay: 3000,
  })

  if (isLoading) return <p className="mt-4 text-xs text-slate-600">Загрузка ссылки...</p>
  if (!data?.subscribe_url) return null

  const url = data.subscribe_url

  const copy = () => {
    navigator.clipboard.writeText(url).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div className="mt-5 border-t border-gray-100 dark:border-surface-700 pt-4">
      <p className="mb-2 text-[10px] font-semibold uppercase tracking-widest text-gray-400 dark:text-slate-600">
        Ссылка для подключения
      </p>
      <div className="flex items-center gap-2 rounded-lg border border-gray-200 dark:border-surface-600 bg-gray-50 dark:bg-surface-800 px-3 py-2">
        <span className="flex-1 truncate font-mono text-xs text-gray-600 dark:text-slate-400">{url}</span>
        <button
          onClick={() => setShowQR((v) => !v)}
          title="Показать QR-код"
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

      <p className="mt-1.5 text-xs text-gray-400 dark:text-slate-600">
        Вставьте в Happ, V2RayN, Hiddify, Streisand или любой совместимый клиент.
      </p>
    </div>
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
  const { data: devicesData } = useQuery({
    queryKey: ['devices'],
    queryFn: devicesApi.list,
  })

  if (profileLoading) return <PageSpinner />

  const activeSub = (subsData?.subscriptions ?? []).find((s) => s.status === 'active' || s.status === 'trial')
  const daysLeft = activeSub ? daysUntil(activeSub.expires_at) : 0

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

      {/* Pending payments — shown after returning from payment page */}
      <PendingPayments />

      {/* Trial period notification banner */}
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

      {/* Stat cards */}
      <div className="grid grid-cols-2 gap-4">
        <StatCard
          label="Баланс ЯДА"
          value={formatYAD(balance?.yad_balance ?? profile?.yad_balance ?? 0)}
          icon={<Icon name="skull" size={28} />}
          accent
        />
        <StatCard
          label="Текущий тариф"
          value={activeSub ? planLabel(activeSub.plan) : 'Нет'}
          sub={activeSub ? (daysLeft > 0 ? `${daysLeft} дней осталось` : 'Подписка закончилась') : 'Нет активной подписки'}
          icon={<Icon name="shield" size={28} />}
        />
      </div>

      {/* Active subscription card */}
      {activeSub && (
        <Card
          title={daysLeft > 0 ? 'Активная подписка' : 'Подписка закончилась'}
          glow
          action={
            <Link to="/subscriptions">
              <Button size="sm" variant="secondary">{daysLeft > 0 ? 'Продлить' : 'Продлить подписку'}</Button>
            </Link>
          }
        >
          {daysLeft === 0 && (
            <div className="mb-4 flex items-center gap-3 rounded-xl border border-red-500/30 bg-red-500/5 px-4 py-3">
              <span className="shrink-0 text-red-500">⏰</span>
              <p className="text-sm text-red-600 dark:text-red-400">
                Подписка закончилась — продлите для продолжения доступа.
              </p>
            </div>
          )}
          <div className="grid grid-cols-2 gap-3 sm:flex sm:flex-wrap sm:gap-6">
            <div>
              <p className="text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:text-slate-600">Тариф</p>
              <p className="mt-1 text-sm font-semibold text-gray-900 dark:text-slate-100">{planLabel(activeSub.plan)}</p>
            </div>
            <div>
              <p className="text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:text-slate-600">Статус</p>
              <div className="mt-1">{subscriptionStatusBadge(activeSub.status)}</div>
            </div>
            <div>
              <p className="text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:text-slate-600">Истекает</p>
              <p className="mt-1 text-sm font-semibold text-gray-900 dark:text-slate-100">{formatDate(activeSub.expires_at)}</p>
            </div>
            <div>
              <p className="text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:text-slate-600">Осталось дней</p>
              <p className={`mt-1 text-sm font-bold ${daysLeft === 0 ? 'text-red-500' : daysLeft <= 3 ? 'text-red-500' : daysLeft <= 7 ? 'text-yellow-500' : 'text-primary-500'}`}>
                {daysLeft > 0 ? daysLeft : 'Закончилась'}
              </p>
            </div>
          </div>

          {/* Traffic stats */}
          {trafficData && (
            <div className="mt-3 border-t border-gray-100 dark:border-surface-700 pt-3">
              <p className="text-xs text-gray-500 dark:text-slate-500">
                Трафик: <span className="font-medium">{formatBytes(trafficData.used_bytes)}</span>
              </p>
            </div>
          )}

          <ConnectionRow />
        </Card>
      )}

      {/* Devices */}
      {activeSub && devicesData && <DeviceList data={devicesData} />}

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

      {/* Quick actions */}
      <Card title="Быстрые действия">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[
            { to: '/subscriptions', icon: 'shield'  as const, label: 'Подписки',  description: 'Купить или продлить' },
            { to: '/referrals',     icon: 'users'   as const, label: 'Рефералы',    description: 'Пригласить друзей' },
            { to: '/shop',          icon: 'shop'    as const, label: 'Магазин',     description: 'Потратить ЯД' },
            { to: '/promo',         icon: 'tag'     as const, label: 'Промокод',   description: 'Активировать код' },
          ].map((action) => (
            <Link
              key={action.to}
              to={action.to}
              className="group flex flex-col gap-2 rounded-xl border border-gray-200 dark:border-surface-700 p-4 text-left transition-all hover:border-primary-900/60 hover:bg-primary-500/5 dark:hover:border-primary-900/60"
            >
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100 dark:bg-surface-800 text-gray-500 dark:text-slate-500 transition-colors group-hover:bg-primary-500/10 group-hover:text-primary-500">
                <Icon name={action.icon} size={16} />
              </div>
              <div>
                <p className="text-sm font-medium text-gray-800 dark:text-slate-200">{action.label}</p>
                <p className="text-xs text-gray-400 dark:text-slate-600">{action.description}</p>
              </div>
            </Link>
          ))}
        </div>
      </Card>
    </div>
  )
}