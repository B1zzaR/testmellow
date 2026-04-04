import apiClient from './client'
import type { Payment } from './types'

export const paymentsApi = {
  listPending: async (): Promise<{ payments: Payment[] }> => {
    const res = await apiClient.get<{ payments: Payment[] }>('/api/payments/pending')
    return res.data
  },

  getById: async (id: string): Promise<Payment> => {
    const res = await apiClient.get<Payment>(`/api/payments/${id}`)
    return res.data
  },

  check: async (id: string): Promise<Payment> => {
    const res = await apiClient.post<Payment>(`/api/payments/${id}/check`)
    return res.data
  },
}
