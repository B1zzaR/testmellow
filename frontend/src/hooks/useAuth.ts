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
      try {
        const profile = await profileApi.getProfile()
        setAuth({
          id: data.user_id,
          is_admin: profile.is_admin,
          email: profile.email ?? null,
        })
        queryClient.invalidateQueries({ queryKey: ['profile'] })
        navigate(profile.is_admin ? '/admin' : '/dashboard')
      } catch {
        setAuth({ id: data.user_id, is_admin: data.is_admin ?? false, email: null })
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
      try {
        const profile = await profileApi.getProfile()
        setAuth({
          id: data.user_id,
          is_admin: profile.is_admin,
          email: profile.email ?? null,
        })
      } catch {
        setAuth({ id: data.user_id, is_admin: data.is_admin ?? false, email: null })
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
    // Call logout endpoint to clear cookies on server
    authApi.logout().catch(() => {
      // Even if logout fails, clear client state
    })

    clearAuth()
    queryClient.clear()
    navigate('/login', { replace: true })
  }
}
