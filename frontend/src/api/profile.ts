import apiClient from './client'
import type { User, BalanceResponse, YADTransaction, TrafficStats } from './types'

export const profileApi = {
  getProfile: async (): Promise<User> => {
    const res = await apiClient.get<User>('/api/profile')
    return res.data
  },

  getConnection: async (): Promise<{ subscribe_url: string }> => {
    const res = await apiClient.get<{ subscribe_url: string }>('/api/profile/connection')
    return res.data
  },

  changePassword: async (oldPassword: string, newPassword: string): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>('/api/profile/password', {
      old_password: oldPassword,
      new_password: newPassword,
    })
    return res.data
  },

  setTelegramID: async (telegramID: number | null, password?: string): Promise<{ message: string; merged: boolean; transferred_yad?: number; transferred_subs?: number }> => {
    const res = await apiClient.put<{ message: string; merged: boolean; transferred_yad?: number; transferred_subs?: number }>('/api/profile/telegram', { telegram_id: telegramID, password: password ?? '' })
    return res.data
  },

  getTraffic: async (): Promise<TrafficStats> => {
    const res = await apiClient.get<TrafficStats>('/api/profile/traffic')
    return res.data
  },

  requestLinkCode: async (): Promise<{ code: string; bot_username: string; expires_in: number }> => {
    const res = await apiClient.post<{ code: string; bot_username: string; expires_in: number }>('/api/profile/telegram/link-code')
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
