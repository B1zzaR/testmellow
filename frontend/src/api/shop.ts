import apiClient from './client'
import type { ShopItem, BuyShopItemRequest, SubscriptionPlan, DeviceExpansionQuote } from './types'

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

  buyDeviceExpansion: async (qty: 1 | 2): Promise<{ message: string; extra_devices: number; expires_at: string }> => {
    const res = await apiClient.post<{ message: string; extra_devices: number; expires_at: string }>(
      '/api/shop/buy-device-expansion',
      { qty },
    )
    return res.data
  },

  buyDeviceExpansionMoney: async (
    qty: 1 | 2,
    return_url: string,
  ): Promise<{ payment_id: string; redirect_url: string }> => {
    const res = await apiClient.post<{ payment_id: string; redirect_url: string }>(
      '/api/shop/buy-device-expansion-money',
      { qty, return_url },
    )
    return res.data
  },

  getDeviceExpansionQuote: async (): Promise<DeviceExpansionQuote> => {
    const res = await apiClient.get<DeviceExpansionQuote>('/api/devices/expansion/quote')
    return res.data
  },
}

