import apiClient from './client'
import type { UsePromoRequest } from './types'

export const promoApi = {
  use: async (data: UsePromoRequest): Promise<{ message: string; yad_earned: number }> => {
    const res = await apiClient.post<{ message: string; yad_earned: number }>(
      '/api/promo/use',
      data,
    )
    return res.data
  },
}
