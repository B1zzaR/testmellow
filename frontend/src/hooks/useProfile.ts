import { useQuery } from '@tanstack/react-query'
import { profileApi } from '@/api/profile'

export function useProfile() {
  return useQuery({
    queryKey: ['profile'],
    queryFn: profileApi.getProfile,
    staleTime: 60_000,
  })
}
