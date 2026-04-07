import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { promoApi } from '@/api/promo'
import { Card } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { Icon } from '@/components/ui/Icons'
import { formatYAD } from '@/utils/formatters'

type Tab = 'yad' | 'discount'

const YAD_HOW_IT_WORKS = [
  'Каждый код можно использовать один раз на аккаунт',
  'Бонусные ЯД начисляются мгновенно на баланс',
  'Используйте ЯД в магазине для активации подписки',
]

const DISCOUNT_HOW_IT_WORKS = [
  'Каждый код скидки можно использовать один раз',
  'Скидка применится автоматически при следующей оплате',
  'Действует на все тарифы в разделе «Продление»',
]

export function PromoPage() {
  const [tab, setTab] = useState<Tab>('yad')
  const [code, setCode] = useState('')
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const queryClient = useQueryClient()

  const { data: activeDiscount } = useQuery({
    queryKey: ['active-discount'],
    queryFn: promoApi.getActiveDiscount,
  })

  const promoMutation = useMutation({
    mutationFn: () => promoApi.use({ code: code.trim().toUpperCase() }),
    onSuccess: (res) => {
      if (res.promo_type === 'yad') {
        queryClient.invalidateQueries({ queryKey: ['balance'] })
        queryClient.invalidateQueries({ queryKey: ['profile'] })
        queryClient.invalidateQueries({ queryKey: ['balance-history'] })
        setSuccessMsg('Промокод применён! Вы получили ' + formatYAD(res.yad_earned) + '.')
      } else {
        queryClient.invalidateQueries({ queryKey: ['active-discount'] })
        setSuccessMsg('Скидка ' + res.discount_percent + '% активирована! Применится при следующей оплате подписки.')
      }
      setCode('')
      setErrorMsg('')
    },
    onError: (e: Error) => {
      setErrorMsg(e.message)
      setSuccessMsg('')
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!code.trim()) return
    promoMutation.mutate()
  }

  const hasActiveDiscount = activeDiscount && activeDiscount.active_discount_percent > 0
  const howItWorks = tab === 'yad' ? YAD_HOW_IT_WORKS : DISCOUNT_HOW_IT_WORKS

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div>
        <h1 className="text-xl font-bold text-gray-900 dark:text-slate-100">Промокод</h1>
        <p className="mt-0.5 text-sm text-gray-500 dark:text-slate-500">Введите код для получения бонусов</p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 rounded-lg border border-surface-700 bg-surface-800 p-1">
        {([
          { key: 'yad' as Tab, label: 'Бонус ЯД', icon: 'skull' as const },
          { key: 'discount' as Tab, label: 'Скидка', icon: 'tag' as const },
        ] as const).map((t) => (
          <button
            key={t.key}
            onClick={() => { setTab(t.key); setSuccessMsg(''); setErrorMsg(''); setCode('') }}
            className={
              'flex flex-1 items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-all ' +
              (tab === t.key
                ? 'bg-primary-500 text-white shadow'
                : 'text-slate-400 hover:text-slate-200')
            }
          >
            <Icon name={t.icon} size={14} />
            {t.label}
          </button>
        ))}
      </div>

      {successMsg && <Alert variant="success" message={successMsg} />}
      {errorMsg && <Alert variant="error" message={errorMsg} />}

      {/* Active discount banner */}
      {tab === 'discount' && hasActiveDiscount && (
        <div className="flex items-center gap-3 rounded-xl border border-primary-500/30 bg-primary-500/10 px-4 py-3">
          <Icon name="tag" size={18} className="text-primary-400 shrink-0" />
          <div>
            <p className="text-sm font-semibold text-primary-300">
              Активная скидка: {activeDiscount.active_discount_percent}%
            </p>
            <p className="text-xs text-slate-400">
              Код: <span className="font-mono">{activeDiscount.active_discount_code}</span> — применится при следующей оплате
            </p>
          </div>
        </div>
      )}

      <Card title={tab === 'yad' ? 'Активировать бонусный код' : 'Активировать код скидки'}>
        {tab === 'discount' && hasActiveDiscount ? (
          <p className="text-sm text-gray-500 dark:text-slate-500">
            Сначала используйте активную скидку <span className="font-mono font-semibold text-primary-400">{activeDiscount.active_discount_code}</span>, затем сможете применить новый промокод.
          </p>
        ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            <Input
              label="Промокод"
              placeholder={tab === 'yad' ? 'напр. BONUS2025' : 'напр. SALE50'}
              value={code}
              onChange={(e) => setCode(e.target.value.toUpperCase())}
            />
            <Button
              type="submit"
              loading={promoMutation.isPending}
              disabled={!code.trim()}
              className="w-full"
            >
              <Icon name="tag" size={14} />
              Применить
            </Button>
          </form>
        )}
      </Card>

      <Card title="Как это работает">
        <ul className="space-y-3">
          {howItWorks.map((item) => (
            <li key={item} className="flex items-start gap-2 text-sm text-gray-600 dark:text-slate-400">
              <Icon name="check" size={14} className="mt-0.5 shrink-0 text-primary-500" />
              {item}
            </li>
          ))}
        </ul>
      </Card>
    </div>
  )
}