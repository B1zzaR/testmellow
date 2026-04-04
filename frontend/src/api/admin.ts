import apiClient from './client'
import type {
  User,
  Ticket,
  TicketMessage,
  PromoCode,
  Analytics,
  ShopItem,
  CreatePromoRequest,
  SetRiskRequest,
  ReplyTicketRequest,
} from './types'

export const adminApi = {
  // ─── Users ──────────────────────────────────────────────────────────────
  listUsers: async (): Promise<{ users: User[] }> => {
    const res = await apiClient.get<{ users: User[] }>('/admin/users')
    return res.data
  },

  getUser: async (id: string): Promise<User> => {
    const res = await apiClient.get<User>(`/admin/users/${id}`)
    return res.data
  },

  banUser: async (id: string): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/admin/users/${id}/ban`)
    return res.data
  },

  unbanUser: async (id: string): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/admin/users/${id}/unban`)
    return res.data
  },

  setRiskScore: async (id: string, data: SetRiskRequest): Promise<{ message: string }> => {
    const res = await apiClient.patch<{ message: string }>(`/admin/users/${id}/risk`, data)
    return res.data
  },

  // ─── Analytics ──────────────────────────────────────────────────────────
  getAnalytics: async (): Promise<Analytics> => {
    const res = await apiClient.get<Analytics>('/admin/analytics')
    return res.data
  },

  // ─── Promo codes ────────────────────────────────────────────────────────
  listPromoCodes: async (): Promise<{ promocodes: PromoCode[] }> => {
    const res = await apiClient.get<{ promocodes: PromoCode[] }>('/admin/promocodes')
    return res.data
  },

  createPromoCode: async (data: CreatePromoRequest): Promise<PromoCode> => {
    const res = await apiClient.post<PromoCode>('/admin/promocodes', data)
    return res.data
  },

  // ─── Tickets ────────────────────────────────────────────────────────────
  listTickets: async (status?: string): Promise<{ tickets: Ticket[] }> => {
    const params = status ? { status } : {}
    const res = await apiClient.get<{ tickets: Ticket[] }>('/admin/tickets', { params })
    return res.data
  },

  getTicket: async (id: string): Promise<{ ticket: Ticket; messages: TicketMessage[] }> => {
    const res = await apiClient.get<{ ticket: Ticket; messages: TicketMessage[] }>(
      `/admin/tickets/${id}`,
    )
    return res.data
  },

  replyToTicket: async (id: string, data: ReplyTicketRequest): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/admin/tickets/${id}/reply`, data)
    return res.data
  },

  closeTicket: async (id: string): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/admin/tickets/${id}/close`)
    return res.data
  },

  // ─── Shop ───────────────────────────────────────────────────────────────
  createShopItem: async (
    data: Omit<ShopItem, 'id' | 'is_active' | 'created_at'>,
  ): Promise<ShopItem> => {
    const res = await apiClient.post<ShopItem>('/admin/shop/items', data)
    return res.data
  },
}
