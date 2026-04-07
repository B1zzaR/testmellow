import { useState, useEffect, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { paymentsApi } from '@/api/payments'
import type { Payment } from '@/api/types'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { formatRubles, planLabel } from '@/utils/formatters'

// ─── Countdown timer hook ────────────────────────────────────────────────────

function useCountdown(expiresAt: string | null): string {
  const [remaining, setRemaining] = useState('')

  useEffect(() => {
    if (!expiresAt) return

    const update = () => {
      const diff = new Date(expiresAt).getTime() - Date.now()
      if (diff <= 0) {
        setRemaining('Истёк')
        return
      }
      const m = Math.floor(diff / 60000)
      const s = Math.floor((diff % 60000) / 1000)
      setRemaining(`${m}:${s.toString().padStart(2, '0')}`)
    }

    update()
    const id = setInterval(update, 1000)
    return () => clearInterval(id)
  }, [expiresAt])

  return remaining
}

// ─── Status badge ─────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: Payment['status'] }) {
  const config: Record<Payment['status'], { label: string; cls: string }> = {
    PENDING:      { label: 'Ожидает',    cls: 'bg-yellow-500/15 text-yellow-400 ring-1 ring-yellow-500/30' },
    CONFIRMED:    { label: 'Оплачено',    cls: 'bg-primary-500/15 text-primary-400 ring-1 ring-primary-500/30' },
    CANCELED:     { label: 'Отменён',  cls: 'bg-red-500/15 text-red-400 ring-1 ring-red-500/30' },
    CHARGEBACKED: { label: 'Возврат', cls: 'bg-orange-500/15 text-orange-400 ring-1 ring-orange-500/30' },
    EXPIRED:      { label: 'Истёк',    cls: 'bg-surface-600/60 text-slate-500 ring-1 ring-surface-600' },
  }
  const { label, cls } = config[status] ?? { label: status, cls: 'bg-surface-600/60 text-slate-500' }
  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${cls}`}>
      {label}
    </span>
  )
}

// ─── Single payment row ───────────────────────────────────────────────────────

interface PaymentRowProps {
  payment: Payment
  onChecked: (updated: Payment) => void
  onRemove: (id: string) => void
}

function PaymentRow({ payment, onChecked, onRemove }: PaymentRowProps) {
  const countdown = useCountdown(payment.expires_at)
  const [dismissed, setDismissed] = useState(false)

  const checkMutation = useMutation({
    mutationFn: () => paymentsApi.check(payment.id),
    onSuccess: (updated) => {
      onChecked(updated)
    },
  })

  // Auto-remove on terminal states after a short delay so the user sees the result.
  useEffect(() => {
    if (
      payment.status === 'CONFIRMED' ||
      payment.status === 'EXPIRED' ||
      payment.status === 'CANCELED' ||
      payment.status === 'CHARGEBACKED'
    ) {
      const timeout = payment.status === 'CONFIRMED' ? 3000 : 1500
      const id = setTimeout(() => {
        setDismissed(true)
        onRemove(payment.id)
      }, timeout)
      return () => clearTimeout(id)
    }
  }, [payment.status, payment.id, onRemove])

  if (dismissed) return null

  const isTerminal = payment.status !== 'PENDING'
  const isExpiredLocally =
    payment.expires_at != null && new Date(payment.expires_at).getTime() < Date.now()

  return (
    <div
      className={`flex flex-col gap-3 rounded-xl border p-4 transition-all sm:flex-row sm:items-center sm:justify-between ${
        payment.status === 'CONFIRMED'
          ? 'border-primary-900/40 bg-primary-500/5'
          : payment.status === 'CANCELED' || payment.status === 'EXPIRED'
            ? 'border-surface-700 bg-surface-900 opacity-60'
            : 'border-surface-700 bg-surface-900'
      }`}
    >
      {/* Left: info */}
      <div className="flex flex-col gap-1">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-slate-100">{planLabel(payment.plan)}</span>
          <StatusBadge status={payment.status} />
        </div>
        <span className="text-sm text-slate-400">{formatRubles(payment.amount_kopecks)}</span>
        {payment.expires_at && !isTerminal && (
          <span
            className={`text-xs ${
              countdown === 'Истёк' ? 'text-red-500' : 'text-gray-400'
            }`}
          >
            {countdown === 'Истёк' ? 'Ссылка устарела' : `Истекает через ${countdown}`}
          </span>
        )}
        {payment.status === 'CONFIRMED' && (
          <span className="text-xs font-medium text-primary-400">
          ✓ Оплата подтверждена — подписка активируется…
          </span>
        )}
      </div>

      {/* Right: actions */}
      <div className="flex shrink-0 items-center gap-2">
        {payment.status === 'PENDING' && payment.redirect_url && !isExpiredLocally && (
          <a
            href={payment.redirect_url}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 rounded-lg border border-primary-500/60 px-3 py-1.5 text-xs font-medium text-primary-400 hover:bg-primary-500/10 transition-colors"
          >
            Оплатить ↗
          </a>
        )}
        {!isTerminal && (
          <Button
            size="sm"
            variant="secondary"
            loading={checkMutation.isPending}
            disabled={checkMutation.isPending}
            onClick={() => checkMutation.mutate()}
          >
            Проверить статус
          </Button>
        )}
      </div>
    </div>
  )
}

// ─── Main component ───────────────────────────────────────────────────────────

interface PendingPaymentsProps {
  /** Called when a payment transitions to CONFIRMED so the parent can reload subscriptions */
  onPaymentConfirmed?: () => void
}

export function PendingPayments({ onPaymentConfirmed }: PendingPaymentsProps) {
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['pendingPayments'],
    queryFn: paymentsApi.listPending,
    // Poll every 15 seconds while the component is mounted
    refetchInterval: 15_000,
    // Keep previous data visible while re-fetching to avoid flicker
    placeholderData: (prev) => prev,
  })

  // Local state so we can add/update payments from manual checks without
  // waiting for the next poll cycle.
  const [localPayments, setLocalPayments] = useState<Payment[]>([])

  useEffect(() => {
    setLocalPayments(data?.payments ?? [])
  }, [data])

  const handleChecked = useCallback(
    (updated: Payment) => {
      setLocalPayments((prev) => prev.map((p) => (p.id === updated.id ? updated : p)))

      if (updated.status === 'CONFIRMED') {
        // Invalidate subscriptions so the active sub appears immediately
        queryClient.invalidateQueries({ queryKey: ['subscriptions'] })
        queryClient.invalidateQueries({ queryKey: ['profile'] })
        onPaymentConfirmed?.()
      }
    },
    [queryClient, onPaymentConfirmed],
  )

  const handleRemove = useCallback((id: string) => {
    setLocalPayments((prev) => prev.filter((p) => p.id !== id))
    queryClient.invalidateQueries({ queryKey: ['pendingPayments'] })
  }, [queryClient])

  if (isLoading) return null
  if (localPayments.length === 0) return null

  return (
    <Card title="Активные платежи" className="border-yellow-900/30">
      <div className="space-y-3">
        {localPayments.map((payment) => (
          <PaymentRow
            key={payment.id}
            payment={payment}
            onChecked={handleChecked}
            onRemove={handleRemove}
          />
        ))}
      </div>
    </Card>
  )
}
