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
import type { PromoCode } from '@/api/types'

export function AdminPromoPage() {
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['admin-promocodes'],
    queryFn: adminApi.listPromoCodes,
  })

  const [modalOpen, setModalOpen] = useState(false)
  const [code, setCode] = useState('')
  const [yadAmount, setYadAmount] = useState('')
  const [maxUses, setMaxUses] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})

  const createMutation = useMutation({
    mutationFn: adminApi.createPromoCode,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-promocodes'] })
      setModalOpen(false)
      setCode('')
      setYadAmount('')
      setMaxUses('')
      setExpiresAt('')
      setFieldErrors({})
    },
    onError: (e: Error) => setErrorMsg(e.message),
  })

  const handleCreate = () => {
    const errs: Record<string, string> = {}
    if (code.trim().length < 4) errs.code = 'Minimum 4 characters'
    const yad = parseInt(yadAmount, 10)
    if (isNaN(yad) || yad < 1) errs.yad = 'Must be at least 1 YAD'
    const uses = parseInt(maxUses, 10)
    if (isNaN(uses) || uses < 1) errs.uses = 'Must be at least 1'
    if (Object.keys(errs).length > 0) { setFieldErrors(errs); return }
    setFieldErrors({})

    createMutation.mutate({
      code: code.trim().toUpperCase(),
      yad_amount: yad,
      max_uses: uses,
      expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
    })
  }

  if (isLoading) return <PageSpinner />

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Promo Codes</h1>
        <Button size="sm" onClick={() => setModalOpen(true)}>
          + Create Code
        </Button>
      </div>

      {errorMsg && <Alert variant="error" message={errorMsg} />}

      <Table<PromoCode>
        keyExtractor={(p) => p.id}
        data={data?.promocodes ?? []}
        emptyMessage="No promo codes yet"
        columns={[
          {
            key: 'code',
            header: 'Code',
            render: (p) => <span className="font-mono font-semibold">{p.code}</span>,
          },
          {
            key: 'yad_amount',
            header: 'YAD Reward',
            render: (p) => formatYAD(p.yad_amount),
          },
          {
            key: 'usage',
            header: 'Usage',
            render: (p) => `${p.used_count} / ${p.max_uses}`,
          },
          {
            key: 'expires_at',
            header: 'Expires',
            render: (p) => (p.expires_at ? formatDate(p.expires_at) : '—'),
          },
          {
            key: 'status',
            header: 'Status',
            render: (p) => {
              const expired = p.expires_at ? new Date(p.expires_at) < new Date() : false
              const exhausted = p.used_count >= p.max_uses
              if (expired || exhausted) return <Badge label="Expired" variant="red" />
              return <Badge label="Active" variant="green" />
            },
          },
          {
            key: 'created_at',
            header: 'Created',
            render: (p) => formatDate(p.created_at),
          },
        ]}
      />

      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title="Create Promo Code"
        footer={
          <>
            <Button variant="secondary" onClick={() => setModalOpen(false)}>
              Cancel
            </Button>
            <Button loading={createMutation.isPending} onClick={handleCreate}>
              Create
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <Input
            label="Code"
            placeholder="SUMMER2025"
            value={code}
            onChange={(e) => setCode(e.target.value.toUpperCase())}
            error={fieldErrors.code}
          />
          <Input
            label="YAD Amount"
            type="number"
            min={1}
            placeholder="50"
            value={yadAmount}
            onChange={(e) => setYadAmount(e.target.value)}
            error={fieldErrors.yad}
          />
          <Input
            label="Max Uses"
            type="number"
            min={1}
            placeholder="100"
            value={maxUses}
            onChange={(e) => setMaxUses(e.target.value)}
            error={fieldErrors.uses}
          />
          <Input
            label="Expires At (optional)"
            type="datetime-local"
            value={expiresAt}
            onChange={(e) => setExpiresAt(e.target.value)}
          />
        </div>
      </Modal>
    </div>
  )
}
