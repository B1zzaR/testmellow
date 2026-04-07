import apiClient from './client'
import type { UsePromoRequest } from './types'

export interface UsePromoResponse {
  message: string
  promo_type: 'yad' | 'discount'
  yad_earned: number
  discount_percent: number
}

export interface ActiveDiscountResponse {
  active_discount_code: string
  active_discount_percent: number
}

export const promoApi = {
  use: async (data: UsePromoRequest): Promise<UsePromoResponse> => {
    const res = await apiClient.post<UsePromoResponse>('/api/promo/use', data)
    return res.data
  },

  getActiveDiscount: async (): Promise<ActiveDiscountResponse> => {
    const res = await apiClient.get<ActiveDiscountResponse>('/api/promo/discount/active')
    return res.data
  },
}
