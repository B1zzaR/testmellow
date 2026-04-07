import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminApi } from '@/api/admin'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { Alert } from '@/components/ui/Alert'
import { Modal } from '@/components/ui/Modal'
import { Table } from '@/components/ui/Table'
import { Badge } from '@/components/ui/Badge'
import { PageSpinner } from '@/components/ui/Spinner'
import { formatDate, formatYAD } from '@/utils/formatters'
import type { PromoCode, PromoType } from '@/api/types'

export function AdminPromoPage() {
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['admin-promocodes'],
    queryFn: adminApi.listPromoCodes,
  })

  const [modalOpen, setModalOpen] = useState(false)
  const [code, setCode] = useState('')
  const [promoType, setPromoType] = useState<PromoType>('yad')
  const [yadAmount, setYadAmount] = useState('')
  const [discountPercent, setDiscountPercent] = useState('')
  const [maxUses, setMaxUses] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})

  const createMutation = useMutation({
    mutationFn: adminApi.createPromoCode,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-promocodes'] })
      setModalOpen(false)
      resetForm()
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const resetForm = () => {
    setCode('')
    setPromoType('yad')
    setYadAmount('')
    setDiscountPercent('')
    setMaxUses('')
    setExpiresAt('')
    setFieldErrors({})
    setErrorMsg('')
  }

  const handleCreate = () => {
    const errs: Record<string, string> = {}
    if (code.trim().length < 4) errs.code = 'Минимум 4 символа'
    if (promoType === 'yad') {
      const yad = parseInt(yadAmount, 10)
      if (isNaN(yad) || yad < 1) errs.yad = 'Не менее 1 ЯД'
    } else {
      const pct = parseInt(discountPercent, 10)
      if (isNaN(pct) || pct < 1 || pct > 100) errs.discount = 'От 1 до 100%'
    }
    const uses = parseInt(maxUses, 10)
    if (isNaN(uses) || uses < 1) errs.uses = 'Не менее 1'
    if (Object.keys(errs).length > 0) { setFieldErrors(errs); return }
    setFieldErrors({})

    createMutation.mutate({
      code: code.trim().toUpperCase(),
      promo_type: promoType,
      yad_amount: promoType === 'yad' ? parseInt(yadAmount, 10) : 0,
      discount_percent: promoType === 'discount' ? parseInt(discountPercent, 10) : 0,
      max_uses: uses,
      expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
    })
  }

  if (isLoading) return <PageSpinner />

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-slate-100">Промокоды</h1>
        <Button size="sm" onClick={() => { resetForm(); setModalOpen(true) }}>
          + Создать код
        </Button>
      </div>

      {errorMsg && <Alert variant="error" message={errorMsg} />}

      <Table<PromoCode>
        keyExtractor={(p) => p.id}
        data={data?.promocodes ?? []}
        emptyMessage="Промокодов пока нет"
        columns={[
          {
            key: 'code',
            header: 'Код',
            render: (p) => <span className="font-mono font-semibold">{p.code}</span>,
          },
          {
            key: 'promo_type',
            header: 'Тип',
            render: (p) =>
              p.promo_type === 'discount'
                ? <Badge label={`Скидка ${p.discount_percent}%`} variant="blue" />
                : <Badge label="ЯД бонус" variant="green" />,
          },
          {
            key: 'value',
            header: 'Значение',
            render: (p) =>
              p.promo_type === 'discount'
                ? `${p.discount_percent}%`
                : formatYAD(p.yad_amount),
          },
          {
            key: 'usage',
            header: 'Применений',
            render: (p) => `${p.used_count} / ${p.max_uses}`,
          },
          {
            key: 'expires_at',
            header: 'Истекает',
            render: (p) => (p.expires_at ? formatDate(p.expires_at) : '—'),
          },
          {
            key: 'status',
            header: 'Статус',
            render: (p) => {
              const expired = p.expires_at ? new Date(p.expires_at) < new Date() : false
              const exhausted = p.used_count >= p.max_uses
              if (expired || exhausted) return <Badge label="Истёк" variant="red" />
              return <Badge label="Активен" variant="green" />
            },
          },
          {
            key: 'created_at',
            header: 'Создан',
            render: (p) => formatDate(p.created_at),
          },
        ]}
      />

      <Modal
        open={modalOpen}
        onClose={() => { setModalOpen(false); resetForm() }}
        title="Создать промокод"
        footer={
          <>
            <Button variant="secondary" onClick={() => { setModalOpen(false); resetForm() }}>
              Отмена
            </Button>
            <Button loading={createMutation.isPending} onClick={handleCreate}>
              Создать
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Код"
            placeholder="SUMMER2025"
            value={code}
            onChange={(e) => setCode(e.target.value.toUpperCase())}
            error={fieldErrors.code}
          />

          {/* Type selector */}
          <div>
            <label className="mb-1.5 block text-sm font-medium text-slate-300">Тип промокода</label>
            <div className="flex gap-2">
              {(['yad', 'discount'] as PromoType[]).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setPromoType(t)}
                  className={`flex-1 rounded-lg border py-2 text-sm font-medium transition-all ${
                    promoType === t
                      ? 'border-primary-500 bg-primary-500/10 text-primary-400'
                      : 'border-surface-600 text-slate-400 hover:border-surface-500'
                  }`}
                >
                  {t === 'yad' ? 'ЯД бонус' : 'Скидка %'}
                </button>
              ))}
            </div>
          </div>

          {promoType === 'yad' ? (
            <Input
              label="Количество ЯД"
              type="number"
              min={1}
              placeholder="50"
              value={yadAmount}
              onChange={(e) => setYadAmount(e.target.value)}
              error={fieldErrors.yad}
            />
          ) : (
            <Input
              label="Скидка (%)"
              type="number"
              min={1}
              max={100}
              placeholder="20"
              value={discountPercent}
              onChange={(e) => setDiscountPercent(e.target.value)}
              error={fieldErrors.discount}
            />
          )}

          <Input
            label="Макс. применений"
            type="number"
            min={1}
            placeholder="100"
            value={maxUses}
            onChange={(e) => setMaxUses(e.target.value)}
            error={fieldErrors.uses}
          />
          <Input
            label="Истекает (необязательно)"
            type="datetime-local"
            value={expiresAt}
            onChange={(e) => setExpiresAt(e.target.value)}
          />
        </div>
      </Modal>
    </div>
  )
}