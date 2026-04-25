import apiClient from './client'
import type { ShopItem, BuyShopItemRequest, SubscriptionPlan } from './types'

export const shopApi = {
  list: async (): Promise<{ items: ShopItem[] }> => {
    const res = await apiClient.get<{ items: ShopItem[] }>('/api/shop')
    return res.data
  },

  buy: async (data: BuyShopItemRequest): Promise<{ message: string; item_id: string }> => {
    const res = await apiClient.post<{ message: string; item_id: string }>('/api/shop/buy', data)
    return res.data
  },

  buySubscription: async (plan: SubscriptionPlan): Promise<{ message: string; expires_at: string }> => {
    const res = await apiClient.post<{ message: string; expires_at: string }>('/api/shop/buy-subscription', { plan })
    return res.data
  },
}

