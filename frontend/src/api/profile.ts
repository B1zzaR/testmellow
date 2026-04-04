import apiClient from './client'
import type { User, BalanceResponse, YADTransaction } from './types'

export const profileApi = {
  getProfile: async (): Promise<User> => {
    const res = await apiClient.get<User>('/api/profile')
    return res.data
  },

  getConnection: async (): Promise<{ subscribe_url: string }> => {
    const res = await apiClient.get<{ subscribe_url: string }>('/api/profile/connection')
    return res.data
  },
}

export const balanceApi = {
  getBalance: async (): Promise<BalanceResponse> => {
    const res = await apiClient.get<BalanceResponse>('/api/balance')
    return res.data
  },

  getHistory: async (): Promise<{ transactions: YADTransaction[] }> => {
    const res = await apiClient.get<{ transactions: YADTransaction[] }>('/api/balance/history')
    return res.data
  },
}
