import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { referralsApi } from '@/api/referrals'
import { Card, StatCard } from '@/components/ui/Card'
import { Alert } from '@/components/ui/Alert'
import { PageSpinner } from '@/components/ui/Spinner'
import { Icon } from '@/components/ui/Icons'
import { formatDate, formatYAD, formatRubles } from '@/utils/formatters'

export function ReferralsPage() {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['referrals'],
    queryFn: referralsApi.list,
  })
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    const code = data?.referral_code
    if (!code) return

    // navigator.clipboard requires HTTPS; fall back to execCommand on HTTP
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(code).then(() => {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      }).catch(() => fallbackCopy(code))
    } else {
      fallbackCopy(code)
    }
  }

  const fallbackCopy = (text: string) => {
    const el = document.createElement('textarea')
    el.value = text
    el.style.cssText = 'position:fixed;opacity:0;top:0;left:0'
    document.body.appendChild(el)
    el.focus()
    el.select()
    try {
      document.execCommand('copy')
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } finally {
      document.body.removeChild(el)
    }
  }

  if (isLoading) return <PageSpinner />
  if (isError) return <Alert variant="error" message="Не удалось загрузить рефералов" />

  const referrals = data?.referrals ?? []
  const totalEarned = referrals.reduce((sum, r) => sum + r.total_reward, 0)

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">Рефералы</h1>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">Приглашайте друзей и получайте 15% от каждого их платежа</p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <StatCard
          label="Всего рефералов"
          value={data?.referral_count ?? 0}
          sub="Приглашённых друзей"
          icon={<Icon name="users" size={28} />}
        />
        <StatCard
          label="Заработано всего"
          value={formatYAD(totalEarned)}
          sub="Начисления ЯД"
          icon={<Icon name="skull" size={28} />}
          accent
        />
      </div>

      {/* Referral code */}
      <Card title="Ваш реферальный код" glow>
        <p className="mb-4 text-sm text-gray-500 dark:text-slate-500">
          Поделитесь кодом с друзьями. Когда они зарегистрируются и оформляют подписку, вы автоматически получаете ЯД.
        </p>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <div className="flex-1 overflow-hidden rounded-lg border border-gray-200 dark:border-surface-600 bg-gray-50 dark:bg-surface-800 px-4 py-3">
            <input
              readOnly
              value={data?.referral_code ?? ''}
              onFocus={(e) => e.target.select()}
              className="w-full bg-transparent font-mono text-sm font-bold tracking-widest text-gray-800 dark:text-slate-200 outline-none sm:text-base sm:tracking-[0.25em]"
            />
          </div>
          <button
            onClick={handleCopy}
            className="flex w-full items-center justify-center gap-1.5 rounded-lg border border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-700 px-4 py-3 text-sm font-medium whitespace-nowrap text-gray-700 dark:text-slate-300 transition-colors hover:bg-gray-50 dark:hover:bg-surface-600 sm:w-auto"
          >
            <Icon name={copied ? 'check' : 'copy'} size={14} className={copied ? 'text-primary-500' : ''} />
            {copied ? 'Скопировано!' : 'Копировать'}
          </button>
        </div>
        <div className="mt-4 rounded-lg bg-primary-500/5 dark:bg-primary-500/10 border border-primary-900/30 px-4 py-3">
          <div className="flex items-start gap-2 text-xs text-primary-700 dark:text-primary-400">
            <Icon name="coins" size={13} className="mt-0.5 shrink-0" />
            <span>
              Вы получаете <strong>15%</strong> от каждого платежа реферала в ЯД. Награда начисляется <strong>сразу</strong> после подтверждения платежа.
            </span>
          </div>
        </div>
      </Card>

      {/* Referred users list */}
      {referrals.length > 0 ? (
        <Card title={`Referred Users (${referrals.length})`}>
          <div className="space-y-2">
            {referrals.map((ref) => (
              <div
                key={ref.id}
                className="flex items-center justify-between rounded-lg border border-gray-100 dark:border-surface-700 px-4 py-3 hover:bg-gray-50 dark:hover:bg-surface-800 transition-colors"
              >
                <div>
                  <p className="text-sm font-medium text-gray-800 dark:text-slate-200">
                    User {ref.referee_id.slice(0, 8)}…
                  </p>
                  <p className="mt-0.5 text-xs text-gray-400 dark:text-slate-600">С {formatDate(ref.created_at)}</p>
                </div>
                <div className="text-right">
                  <p className="text-sm font-bold text-primary-500">+{formatYAD(ref.total_reward)}</p>
                  <p className="mt-0.5 text-xs text-gray-400 dark:text-slate-600">LTV: {formatRubles(ref.total_paid_ltv)}</p>
                </div>
              </div>
            ))}
          </div>
        </Card>
      ) : (
        <div className="rounded-xl border border-dashed border-gray-300 dark:border-surface-600 py-12 text-center">
          <Icon name="users" size={32} className="mx-auto mb-3 text-gray-300 dark:text-slate-700" />
          <p className="text-sm font-medium text-gray-500 dark:text-slate-500">Нет рефералов</p>
          <p className="mt-1 text-xs text-gray-400 dark:text-slate-600">Поделитесь кодом, чтобы начать зарабатывать</p>
        </div>
      )}
    </div>
  )
}

