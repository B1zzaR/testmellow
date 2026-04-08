import apiClient from './client'
import type { DeviceListResponse } from './types'

export const devicesApi = {
  list: async (): Promise<DeviceListResponse> => {
    const res = await apiClient.get<DeviceListResponse>('/api/devices')
    return res.data
  },

  register: async (hwid: string, deviceName: string): Promise<{ id: string; device_name: string; last_active: string; is_active: boolean; is_inactive: boolean }> => {
    const res = await apiClient.post('/api/devices/register', {
      hwid,
      device_name: deviceName,
    })
    return res.data
  },

  disconnect: async (deviceId: string): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/api/devices/${deviceId}/disconnect`)
    return res.data
  },
}
