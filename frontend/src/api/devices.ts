import apiClient from './client'
import type { DeviceListResponse } from './types'

export const devicesApi = {
  list: async (): Promise<DeviceListResponse> => {
    const res = await apiClient.get<DeviceListResponse>('/api/devices')
    return res.data
  },

  disconnect: async (deviceId: string): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/api/devices/${deviceId}/disconnect`)
    return res.data
  },
}
