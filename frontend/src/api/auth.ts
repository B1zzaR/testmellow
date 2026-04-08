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

  refresh: async (): Promise<void> => {
    // Cookies are sent automatically via withCredentials — no body needed.
    await apiClient.post('/api/auth/refresh')
  },

  logout: async (): Promise<void> => {
    // Call logout endpoint to clear cookies on server
    await apiClient.post('/api/auth/logout')
  },
}
