import apiClient from './client'

export const suggestionsApi = {
  submit: async (body: string): Promise<{ id: string; message: string }> => {
    const res = await apiClient.post<{ id: string; message: string }>('/api/suggestions', { body })
    return res.data
  },
}
