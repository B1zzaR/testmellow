import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { shopApi } from '@/api/shop'
import { Card } from '@/components/ui/Card'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Alert'
import { Modal } from '@/components/ui/Modal'
import { PageSpinner } from '@/components/ui/Spinner'
import { formatYAD } from '@/utils/formatters'
import type { ShopItem } from '@/api/types'

export function ShopPage() {
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['shop'],
    queryFn: shopApi.list,
  })
  const [buyItem, setBuyItem] = useState<ShopItem | null>(null)
  const [successMsg, setSuccessMsg] = useState('')
  const [errorMsg, setErrorMsg] = useState('')

  const buyMutation = useMutation({
    mutationFn: (itemId: string) => shopApi.buy({ item_id: itemId }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['balance'] })
      queryClient.invalidateQueries({ queryKey: ['profile'] })
      queryClient.invalidateQueries({ queryKey: ['shop'] })
      setSuccessMsg(`Successfully purchased: ${buyItem?.name}`)
      setBuyItem(null)
    },
    onError: (e: Error) => {
      setBuyItem(null)
      setErrorMsg(e.message)
    },
  })

  if (isLoading) return <PageSpinner />

  const items = (data?.items ?? []).filter((i) => i.is_active)

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Shop</h1>
      <p className="text-sm text-gray-500">Spend your YAD balance on exclusive items.</p>

      {successMsg && <Alert variant="success" message={successMsg} />}
      {errorMsg && <Alert variant="error" message={errorMsg} />}

      {items.length === 0 ? (
        <div className="rounded-xl border border-dashed border-gray-300 px-6 py-10 text-center">
          <p className="text-gray-400">No items available right now</p>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {items.map((item) => (
            <Card key={item.id}>
              <div className="flex h-full flex-col">
                <h3 className="text-base font-semibold text-gray-900">{item.name}</h3>
                {item.description && (
                  <p className="mt-1 flex-1 text-sm text-gray-500">{item.description}</p>
                )}
                <div className="mt-4 flex items-center justify-between">
                  <span className="text-lg font-bold text-primary-600">
                    {formatYAD(item.price_yad)}
                  </span>
                  {item.stock !== -1 && (
                    <span className="text-xs text-gray-400">{item.stock} left</span>
                  )}
                </div>
                <Button
                  className="mt-3 w-full"
                  size="sm"
                  onClick={() => setBuyItem(item)}
                  disabled={item.stock === 0}
                >
                  {item.stock === 0 ? 'Out of Stock' : 'Purchase'}
                </Button>
              </div>
            </Card>
          ))}
        </div>
      )}

      <Modal
        open={Boolean(buyItem)}
        onClose={() => setBuyItem(null)}
        title="Confirm Purchase"
        footer={
          <>
            <Button variant="secondary" onClick={() => setBuyItem(null)}>
              Cancel
            </Button>
            <Button
              loading={buyMutation.isPending}
              onClick={() => buyItem && buyMutation.mutate(buyItem.id)}
            >
              Confirm
            </Button>
          </>
        }
      >
        {buyItem && (
          <div className="space-y-3">
            <p className="text-gray-700">
              Are you sure you want to purchase <strong>{buyItem.name}</strong>?
            </p>
            <p className="text-gray-600">
              Cost: <strong>{formatYAD(buyItem.price_yad)}</strong>
            </p>
          </div>
        )}
      </Modal>
    </div>
  )
}
