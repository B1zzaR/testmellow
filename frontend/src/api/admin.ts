import apiClient from './client'
import type {
  User,
  Ticket,
  TicketMessage,
  PromoCode,
  Analytics,
  ShopItem,
  Payment,
  Subscription,
  YADTransaction,
  Referral,
  AdminAuditLog,
  RevenueAnalytics,
  CreatePromoRequest,
  SetRiskRequest,
  ReplyTicketRequest,
  AdjustYADRequest,
  AdminAdjustYADRequest,
  SetSubscriptionStatusRequest,
  ExtendSubscriptionRequest,
  CheckPaymentStatusResponse,
} from './types'

export const adminApi = {
  // ─── Users ──────────────────────────────────────────────────────────────
  listUsers: async (params?: { q?: string; page?: number }): Promise<{ users: User[]; total: number; page: number; limit: number }> => {
    const res = await apiClient.get<{ users: User[]; total: number; page: number; limit: number }>('/api/admin/users', { params })
    return res.data
  },

  getUser: async (id: string): Promise<User> => {
    const res = await apiClient.get<User>(`/api/admin/users/${id}`)
    return res.data
  },

  banUser: async (id: string): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/api/admin/users/${id}/ban`)
    return res.data
  },

  unbanUser: async (id: string): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/api/admin/users/${id}/unban`)
    return res.data
  },

  setRiskScore: async (id: string, data: SetRiskRequest): Promise<{ message: string }> => {
    const res = await apiClient.patch<{ message: string }>(`/api/admin/users/${id}/risk`, data)
    return res.data
  },

  // ─── Analytics ──────────────────────────────────────────────────────────
  getAnalytics: async (): Promise<Analytics> => {
    const res = await apiClient.get<Analytics>('/api/admin/analytics')
    return res.data
  },

  // ─── Promo codes ────────────────────────────────────────────────────────
  listPromoCodes: async (): Promise<{ promocodes: PromoCode[] }> => {
    const res = await apiClient.get<{ promocodes: PromoCode[] }>('/api/admin/promocodes')
    return res.data
  },

  createPromoCode: async (data: CreatePromoRequest): Promise<PromoCode> => {
    const res = await apiClient.post<PromoCode>('/api/admin/promocodes', data)
    return res.data
  },

  // ─── Tickets ────────────────────────────────────────────────────────────
  listTickets: async (status?: string): Promise<{ tickets: Ticket[] }> => {
    const params = status ? { status } : {}
    const res = await apiClient.get<{ tickets: Ticket[] }>('/api/admin/tickets', { params })
    return res.data
  },

  getTicket: async (id: string): Promise<{ ticket: Ticket; messages: TicketMessage[] }> => {
    const res = await apiClient.get<{ ticket: Ticket; messages: TicketMessage[] }>(
      `/api/admin/tickets/${id}`,
    )
    return res.data
  },

  replyToTicket: async (id: string, data: ReplyTicketRequest): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/api/admin/tickets/${id}/reply`, data)
    return res.data
  },

  closeTicket: async (id: string): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/api/admin/tickets/${id}/close`)
    return res.data
  },

  // ─── Shop ───────────────────────────────────────────────────────────────
  createShopItem: async (
    data: Omit<ShopItem, 'id' | 'is_active' | 'created_at'>,
  ): Promise<ShopItem> => {
    const res = await apiClient.post<ShopItem>('/api/admin/shop/items', data)
    return res.data
  },

  // ─── Payments ───────────────────────────────────────────────────────────
  listPayments: async (params?: {
    status?: string
    from?: string
    to?: string
    limit?: number
    offset?: number
  }): Promise<{ payments: Payment[]; total: number; limit: number; offset: number }> => {
    const res = await apiClient.get<{ payments: Payment[]; total: number; limit: number; offset: number }>('/api/admin/payments', { params })
    return res.data
  },

  getPayment: async (id: string): Promise<Payment> => {
    const res = await apiClient.get<Payment>(`/api/admin/payments/${id}`)
    return res.data
  },

  checkPaymentStatus: async (id: string): Promise<CheckPaymentStatusResponse> => {
    const res = await apiClient.post<CheckPaymentStatusResponse>(`/api/admin/payments/${id}/check`)
    return res.data
  },

  // ─── Subscriptions ───────────────────────────────────────────────────────
  listSubscriptions: async (params?: {
    status?: string
    user_id?: string
    limit?: number
    offset?: number
  }): Promise<{ subscriptions: Subscription[]; total: number; limit: number; offset: number }> => {
    const res = await apiClient.get<{ subscriptions: Subscription[]; total: number; limit: number; offset: number }>('/api/admin/subscriptions', { params })
    return res.data
  },

  setSubscriptionStatus: async (
    id: string,
    data: SetSubscriptionStatusRequest,
  ): Promise<{ message: string }> => {
    const res = await apiClient.patch<{ message: string }>(`/api/admin/subscriptions/${id}/status`, data)
    return res.data
  },

  extendSubscription: async (
    id: string,
    data: ExtendSubscriptionRequest,
  ): Promise<{ subscription: Subscription; message: string }> => {
    const res = await apiClient.post<{ subscription: Subscription; message: string }>(
      `/api/admin/subscriptions/${id}/extend`,
      data,
    )
    return res.data
  },

  // ─── YAD economy ────────────────────────────────────────────────────────
  listAllYADTransactions: async (params?: {
    login?: string
    type?: string
    limit?: number
    offset?: number
  }): Promise<{ transactions: YADTransaction[] }> => {
    const res = await apiClient.get<{ transactions: YADTransaction[] }>('/api/admin/yad', { params })
    return res.data
  },

  adminAdjustYAD: async (data: AdminAdjustYADRequest): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>('/api/admin/yad/adjust', data)
    return res.data
  },

  // ─── Referrals ───────────────────────────────────────────────────────────
  listAllReferrals: async (params?: {
    login?: string
    limit?: number
    offset?: number
  }): Promise<{ referrals: Referral[] }> => {
    const res = await apiClient.get<{ referrals: Referral[] }>('/api/admin/referrals', { params })
    return res.data
  },

  // ─── User sub-resources ──────────────────────────────────────────────────
  getUserSubscriptions: async (id: string): Promise<{ subscriptions: Subscription[] }> => {
    const res = await apiClient.get<{ subscriptions: Subscription[] }>(
      `/api/admin/users/${id}/subscriptions`,
    )
    return res.data
  },

  getUserPayments: async (id: string): Promise<{ payments: Payment[] }> => {
    const res = await apiClient.get<{ payments: Payment[] }>(`/api/admin/users/${id}/payments`)
    return res.data
  },

  getUserYAD: async (id: string): Promise<{ transactions: YADTransaction[] }> => {
    const res = await apiClient.get<{ transactions: YADTransaction[] }>(`/api/admin/users/${id}/yad`)
    return res.data
  },

  adjustUserYAD: async (id: string, data: AdjustYADRequest): Promise<{ message: string }> => {
    const res = await apiClient.post<{ message: string }>(`/api/admin/users/${id}/adjust-yad`, data)
    return res.data
  },

  // ─── Analytics ───────────────────────────────────────────────────────────
  getRevenueAnalytics: async (days = 30): Promise<RevenueAnalytics> => {
    const res = await apiClient.get<RevenueAnalytics>('/api/admin/analytics/revenue', {
      params: { days },
    })
    return res.data
  },

  // ─── Audit logs ──────────────────────────────────────────────────────────
  listAuditLogs: async (params?: {
    limit?: number
    offset?: number
  }): Promise<{ logs: AdminAuditLog[] }> => {
    const res = await apiClient.get<{ logs: AdminAuditLog[] }>('/api/admin/audit-logs', { params })
    return res.data
  },

  // ─── Manual subscription assignment ──────────────────────────────────────
  assignSubscription: async (data: {
    login: string
    plan: string
  }): Promise<{ message: string; subscription: Subscription; expires_at: string; login: string }> => {
    const res = await apiClient.post<{
      message: string
      subscription: Subscription
      expires_at: string
      login: string
    }>('/api/admin/subscriptions/assign', data)
    return res.data
  },
}
