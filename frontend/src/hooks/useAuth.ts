import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { authApi } from '@/api/auth'
import { profileApi } from '@/api/profile'
import { useAuthStore } from '@/store/authStore'
import type { RegisterRequest } from '@/api/types'

export function useLogin() {
  const { setAuth } = useAuthStore()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: authApi.login,
    onSuccess: async (data) => {
      // Fetch profile to get is_admin and email after login
      localStorage.setItem('auth_token', data.token)
      try {
        const profile = await profileApi.getProfile()
        setAuth(data.token, {
          id: data.user_id,
          is_admin: profile.is_admin,
          email: profile.email ?? null,
        })
        queryClient.invalidateQueries({ queryKey: ['profile'] })
        navigate(profile.is_admin ? '/admin' : '/dashboard')
      } catch {
        setAuth(data.token, { id: data.user_id, is_admin: false, email: null })
        navigate('/dashboard')
      }
    },
  })
}

export function useRegister() {
  const { setAuth } = useAuthStore()
  const navigate = useNavigate()

  return useMutation({
    mutationFn: (data: RegisterRequest) => authApi.register(data),
    onSuccess: async (data) => {
      localStorage.setItem('auth_token', data.token)
      try {
        const profile = await profileApi.getProfile()
        setAuth(data.token, {
          id: data.user_id,
          is_admin: profile.is_admin,
          email: profile.email ?? null,
        })
      } catch {
        setAuth(data.token, { id: data.user_id, is_admin: false, email: null })
      }
      navigate('/dashboard')
    },
  })
}

export function useLogout() {
  const { clearAuth } = useAuthStore()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  return () => {
    clearAuth()
    queryClient.clear()
    navigate('/login')
  }
}
