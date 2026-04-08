import axios, { AxiosError, type AxiosRequestConfig, type AxiosResponse } from 'axios'
import type { ApiError } from './types'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? ''

const MAX_RETRIES = 2
const RETRY_DELAY_MS = 500

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

// ─── Helpers ──────────────────────────────────────────────────────────────────
let _refreshing: Promise<string> | null = null

async function tryRefresh(): Promise<string> {
  if (_refreshing) return _refreshing
  _refreshing = (async () => {
    const refreshToken = localStorage.getItem('refresh_token')
    if (!refreshToken) throw new Error('no refresh token')
    const res = await axios.post<{ token: string; refresh_token: string }>(
      `${BASE_URL}/api/auth/refresh`,
      { refresh_token: refreshToken },
    )
    const { token, refresh_token: newRefresh } = res.data
    localStorage.setItem('auth_token', token)
    if (newRefresh) localStorage.setItem('refresh_token', newRefresh)
    return token
  })()
  _refreshing.finally(() => { _refreshing = null })
  return _refreshing
}

// ─── Response interceptor: normalize errors + retry on network failures ──────
apiClient.interceptors.response.use(
  (response: AxiosResponse) => response,
  async (error: AxiosError<ApiError>) => {
    const config = error.config as AxiosRequestConfig & { _retryCount?: number; _refreshed?: boolean }

    // Retry only on network errors (no response) — never on 4xx/5xx
    const isNetworkError = !error.response && Boolean(error.code !== 'ECONNABORTED')
    if (isNetworkError && config) {
      config._retryCount = (config._retryCount ?? 0) + 1
      if (config._retryCount <= MAX_RETRIES) {
        await new Promise((r) => setTimeout(r, RETRY_DELAY_MS * config._retryCount!))
        return apiClient(config)
      }
    }

    // On 401: try refresh token once before giving up
    if (error.response?.status === 401 && config && !config._refreshed) {
      config._refreshed = true
      try {
        const newToken = await tryRefresh()
        config.headers = { ...(config.headers ?? {}), Authorization: `Bearer ${newToken}` }
        return apiClient(config)
      } catch {
        // Refresh failed — clear session and redirect
        localStorage.removeItem('auth_token')
        localStorage.removeItem('refresh_token')
        localStorage.removeItem('auth_user')
        if (window.location.pathname !== '/login') {
          window.location.href = '/login'
        }
      }
    }

    if (error.response?.status === 401) {
      localStorage.removeItem('auth_token')
      localStorage.removeItem('refresh_token')
      localStorage.removeItem('auth_user')
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }

    const raw =
      error.response?.data?.error ??
      error.message ??
      'Произошла непредвиденная ошибка'

    let message = raw
    if (/Field validation for|Key:.*Error:/.test(raw)) {
      message = 'Проверьте правильность заполнения формы'
    } else if (/Network Error|ERR_NETWORK|Failed to fetch/.test(raw)) {
      message = 'Ошибка сети — проверьте подключение к интернету'
    } else if (/timeout|ECONNABORTED/.test(raw)) {
      message = 'Превышено время ожидания — попробуйте позже'
    } else if (error.response?.status === 429) {
      const retryAfter = error.response.headers['retry-after']
      message = retryAfter
        ? `Слишком много запросов — повторите через ${retryAfter} сек.`
        : 'Слишком много запросов — повторите позже'
    }

    return Promise.reject(new Error(message))
  },
)

export default apiClient
