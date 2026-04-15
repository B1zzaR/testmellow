import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { authApi } from '@/api/auth'
import { profileApi } from '@/api/profile'
import { useAuthStore } from '@/store/authStore'
import type { RegisterRequest } from '@/api/types'
import { useState, useRef, useCallback, useEffect } from 'react'

export function useLogin() {
  const { setAuth } = useAuthStore()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [tfaState, setTfaState] = useState<{
    required: boolean
    challengeId: string | null
    status: 'pending' | 'denied' | 'expired' | null
  }>({ required: false, challengeId: null, status: null })
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const stopPolling = useCallback(() => {
    if (pollingRef.current) {
      clearInterval(pollingRef.current)
      pollingRef.current = null
    }
  }, [])

  const loginMutation = useMutation({
    mutationFn: authApi.login,
    onSuccess: async (data) => {
      if (data.tfa_required && data.challenge_id) {
        setTfaState({ required: true, challengeId: data.challenge_id, status: 'pending' })
        // Start polling
        stopPolling()
        pollingRef.current = setInterval(async () => {
          try {
            const result = await authApi.checkTFA(data.challenge_id!)
            if (result.status === 'approved') {
              stopPolling()
              setTfaState({ required: false, challengeId: null, status: null })
              try {
                const profile = await profileApi.getProfile()
                setAuth({
                  id: result.user_id!,
                  is_admin: profile.is_admin,
                  email: profile.email ?? null,
                })
                queryClient.invalidateQueries({ queryKey: ['profile'] })
                navigate(profile.is_admin ? '/admin' : '/dashboard')
              } catch {
                setAuth({ id: result.user_id!, is_admin: result.is_admin ?? false, email: null })
                navigate('/dashboard')
              }
            } else if (result.status === 'denied') {
              stopPolling()
              setTfaState(s => ({ ...s, status: 'denied' }))
            } else if (result.status === 'expired') {
              stopPolling()
              setTfaState(s => ({ ...s, status: 'expired' }))
            }
          } catch {
            stopPolling()
            setTfaState(s => ({ ...s, status: 'expired' }))
          }
        }, 3000)
        return
      }
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

  const resetTFA = useCallback(() => {
    stopPolling()
    setTfaState({ required: false, challengeId: null, status: null })
  }, [stopPolling])

  // Cleanup on unmount
  useEffect(() => stopPolling, [stopPolling])

  return { ...loginMutation, tfaState, resetTFA }
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
