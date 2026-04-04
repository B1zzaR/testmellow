import apiClient from './client'
import type { AuthResponse, LoginRequest, RegisterRequest } from './types'

export const authApi = {
  login: async (data: LoginRequest): Promise<AuthResponse> => {
    const res = await apiClient.post<AuthResponse>('/api/auth/login', data)
    return res.data
  },

  register: async (data: RegisterRequest): Promise<AuthResponse> => {
    const res = await apiClient.post<AuthResponse>('/api/auth/register', data)
    return res.data
  },
}
