import apiClient from './client'
import type { ShopItem, BuyShopItemRequest, SubscriptionPlan } from './types'

export interface DeviceExpansionQuote {
  qty: number
  price_kopecks: number
  price_yad: number
  remaining_days: number
  expires_at: string
  current_extra: number
  new_total: number
}

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

  quoteDeviceExpansion: async (qty: 1 | 2): Promise<DeviceExpansionQuote> => {
    const res = await apiClient.get<DeviceExpansionQuote>(`/api/devices/expansion/quote?qty=${qty}`)
    return res.data
  },

  buyDeviceExpansion: async (qty: 1 | 2 = 1): Promise<{ message: string; extra_devices: number; expires_at: string; total_limit: number }> => {
    const res = await apiClient.post<{ message: string; extra_devices: number; expires_at: string; total_limit: number }>('/api/shop/buy-device-expansion', { qty })
    return res.data
  },

  buyDeviceExpansionMoney: async (qty: 1 | 2, returnUrl: string): Promise<{ payment_id: string; redirect_url: string; amount_rub: number; qty: number; expires_in: string }> => {
    const res = await apiClient.post<{ payment_id: string; redirect_url: string; amount_rub: number; qty: number; expires_in: string }>('/api/shop/buy-device-expansion-money', { qty, return_url: returnUrl })
    return res.data
  },

  extendDeviceExpansion: async (): Promise<{ message: string; extra_devices: number; expires_at: string; total_limit: number }> => {
    const res = await apiClient.post<{ message: string; extra_devices: number; expires_at: string; total_limit: number }>('/api/shop/extend-device-expansion', {})
    return res.data
  },
}

