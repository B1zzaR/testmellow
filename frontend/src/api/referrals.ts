import apiClient from './client'
import type { ReferralsResponse } from './types'

export const referralsApi = {
  list: async (): Promise<ReferralsResponse> => {
    const res = await apiClient.get<ReferralsResponse>('/api/referrals')
    return res.data
  },
}
