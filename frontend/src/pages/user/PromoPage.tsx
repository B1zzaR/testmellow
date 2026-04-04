import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { promoApi } from '@/api/promo'
import { Card } from '@/components/ui/Card'
import { Input } from '@/components/ui/Input'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { formatYAD } from '@/utils/formatters'

export function PromoPage() {
  const [code, setCode] = useState('')
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const queryClient = useQueryClient()

  const promoMutation = useMutation({
    mutationFn: () => promoApi.use({ code: code.trim().toUpperCase() }),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['balance'] })
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      queryClient.invalidateQueries({ queryKey: ['balance-history'] })
      setSuccessMsg(`Promo applied! You received ${formatYAD(res.yad_earned)}.`)
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

  return (
    <div className="mx-auto max-w-md space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Promo Code</h1>

      {successMsg && <Alert variant="success" message={successMsg} />}
      {errorMsg && <Alert variant="error" message={errorMsg} />}

      <Card title="Redeem Promo Code">
        <p className="mb-4 text-sm text-gray-500">
          Enter your promo code below to receive YAD bonus credits.
        </p>
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            label="Promo Code"
            placeholder="e.g. SUMMER2025"
            value={code}
            onChange={(e) => setCode(e.target.value.toUpperCase())}
          />
          <Button
            type="submit"
            loading={promoMutation.isPending}
            disabled={!code.trim()}
            className="w-full"
          >
            Apply Code
          </Button>
        </form>
      </Card>

      <Card title="How it works" className="bg-blue-50 border-blue-100">
        <ul className="space-y-2 text-sm text-blue-800">
          <li>✓ Each code can be used once per account</li>
          <li>✓ Promo credits are added instantly to your YAD balance</li>
          <li>✓ Use YAD balance in the Shop or for discounts</li>
        </ul>
      </Card>
    </div>
  )
}
