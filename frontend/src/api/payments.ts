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

  listHistory: async (page = 1, perPage = 10): Promise<{ payments: Payment[]; total: number }> => {
    const res = await apiClient.get<{ payments: Payment[]; total: number }>(
      `/api/payments/history?page=${page}&per_page=${perPage}`,
    )
    return res.data
  },
}
