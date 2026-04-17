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

  buyDeviceExpansion: async (): Promise<{ message: string; extra_devices: number; expires_at: string; total_limit: number }> => {
    const res = await apiClient.post<{ message: string; extra_devices: number; expires_at: string; total_limit: number }>('/api/shop/buy-device-expansion', {})
    return res.data
  },

  buyDeviceExpansionMoney: async (returnUrl: string): Promise<{ payment_id: string; redirect_url: string; amount_rub: number; expires_in: string }> => {
    const res = await apiClient.post<{ payment_id: string; redirect_url: string; amount_rub: number; expires_in: string }>('/api/shop/buy-device-expansion-money', { return_url: returnUrl })
    return res.data
  },

  extendDeviceExpansion: async (): Promise<{ message: string; extra_devices: number; expires_at: string; total_limit: number }> => {
    const res = await apiClient.post<{ message: string; extra_devices: number; expires_at: string; total_limit: number }>('/api/shop/extend-device-expansion', {})
    return res.data
  },

  extendDeviceExpansionMoney: async (returnUrl: string): Promise<{ payment_id: string; redirect_url: string; amount_rub: number; expires_in: string }> => {
    const res = await apiClient.post<{ payment_id: string; redirect_url: string; amount_rub: number; expires_in: string }>('/api/shop/extend-device-expansion-money', { return_url: returnUrl })
    return res.data
  },
}
