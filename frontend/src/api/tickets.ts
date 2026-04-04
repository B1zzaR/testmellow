import apiClient from './client'
import type { Ticket, TicketMessage, CreateTicketRequest, ReplyTicketRequest } from './types'

export const ticketsApi = {
  list: async (): Promise<{ tickets: Ticket[] }> => {
    const res = await apiClient.get<{ tickets: Ticket[] }>('/api/tickets')
    return res.data
  },

  getById: async (id: string): Promise<{ ticket: Ticket; messages: TicketMessage[] }> => {
    const res = await apiClient.get<{ ticket: Ticket; messages: TicketMessage[] }>(
      `/api/tickets/${id}`,
    )
    return res.data
  },

  create: async (data: CreateTicketRequest): Promise<Ticket> => {
    const res = await apiClient.post<Ticket>('/api/tickets', data)
    return res.data
  },

  reply: async (id: string, data: ReplyTicketRequest): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/api/tickets/${id}/reply`, data)
    return res.data
  },
}
