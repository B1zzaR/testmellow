import axios, { AxiosError, type AxiosResponse } from 'axios'
import type { ApiError } from './types'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? ''

export const apiClient = axios.create({
  baseURL: BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 15_000,
})

// ─── Request interceptor: inject JWT ─────────────────────────────────────────
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// ─── Response interceptor: normalize errors ──────────────────────────────────
apiClient.interceptors.response.use(
  (response: AxiosResponse) => response,
  (error: AxiosError<ApiError>) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('auth_token')
      localStorage.removeItem('auth_user')
      // Redirect to login without hard refresh when possible
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }
    const raw =
      error.response?.data?.error ??
      error.message ??
      'Произошла непредвиденная ошибка'
    // Translate technical gin-validator errors and network errors
    let message = raw
    if (/Field validation for|Key:.*Error:/.test(raw)) {
      message = 'Проверьте правильность заполнения формы'
    } else if (/Network Error|ERR_NETWORK|Failed to fetch/.test(raw)) {
      message = 'Ошибка сети — проверьте подключение к интернету'
    } else if (/timeout/.test(raw)) {
      message = 'Превышено время ожидания — попробуйте позже'
    }
    return Promise.reject(new Error(message))
  },
)

export default apiClient
