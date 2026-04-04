import apiClient from './client'
import type {
  Subscription,
  BuySubscriptionRequest,
  BuySubscriptionResponse,
} from './types'

export const subscriptionsApi = {
  list: async (): Promise<{ subscriptions: Subscription[] }> => {
    const res = await apiClient.get<{ subscriptions: Subscription[] }>('/api/subscriptions')
    return res.data
  },

  getById: async (id: string): Promise<Subscription> => {
    const res = await apiClient.get<Subscription>(`/api/subscriptions/${id}`)
    return res.data
  },

  buy: async (data: BuySubscriptionRequest): Promise<BuySubscriptionResponse> => {
    const res = await apiClient.post<BuySubscriptionResponse>('/api/subscriptions/buy', data)
    return res.data
  },

  renew: async (data: BuySubscriptionRequest): Promise<BuySubscriptionResponse> => {
    const res = await apiClient.post<BuySubscriptionResponse>('/api/subscriptions/renew', data)
    return res.data
  },

  activateTrial: async (): Promise<{ message: string; expires_at: string; status: string }> => {
    const res = await apiClient.post<{ message: string; expires_at: string; status: string }>(
      '/api/trial/activate',
    )
    return res.data
  },
}
