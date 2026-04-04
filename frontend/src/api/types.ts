// ─── Domain types matching backend domain models ──────────────────────────────

export type PaymentStatus = 'PENDING' | 'CONFIRMED' | 'CANCELED' | 'CHARGEBACKED' | 'EXPIRED'
export type SubscriptionStatus = 'active' | 'expired' | 'trial' | 'canceled'
export type SubscriptionPlan = '1week' | '1month' | '3months'
export type YADTxType = 'referral_reward' | 'bonus' | 'spent' | 'promo' | 'trial'
export type TicketStatus = 'open' | 'answered' | 'closed'
export type RewardSplitStatus = 'pending' | 'immediate' | 'deferred' | 'paid' | 'blocked'

export interface User {
  id: string
  email: string | null
  username: string | null
  yad_balance: number
  referral_code: string
  ltv_kopecks: number
  trial_used: boolean
  is_admin: boolean
  is_banned: boolean
  risk_score: number
  created_at: string
  updated_at: string
}

export interface Subscription {
  id: string
  user_id: string
  plan: SubscriptionPlan
  status: SubscriptionStatus
  starts_at: string
  expires_at: string
  remna_sub_uuid: string | null
  paid_kopecks: number
  payment_id: string | null
  created_at: string
  updated_at: string
}

export interface Payment {
  id: string
  user_id: string
  amount_kopecks: number
  currency: string
  status: PaymentStatus
  plan: SubscriptionPlan
  redirect_url: string
  expires_at: string | null
  created_at: string
  updated_at: string
}

export interface Referral {
  id: string
  referrer_id: string
  referee_id: string
  total_paid_ltv: number
  total_reward: number
  created_at: string
}

export interface YADTransaction {
  id: string
  user_id: string
  delta: number
  balance: number
  tx_type: YADTxType
  ref_id: string | null
  note: string
  created_at: string
}

export interface PromoCode {
  id: string
  code: string
  yad_amount: number
  max_uses: number
  used_count: number
  expires_at: string | null
  created_by_id: string
  created_at: string
}

export interface Ticket {
  id: string
  user_id: string
  subject: string
  status: TicketStatus
  created_at: string
  updated_at: string
}

export interface TicketMessage {
  id: string
  ticket_id: string
  sender_id: string
  is_admin: boolean
  body: string
  created_at: string
}

export interface ShopItem {
  id: string
  name: string
  description: string
  price_yad: number
  stock: number
  is_active: boolean
  created_at: string
}

export interface Analytics {
  total_users: number
  active_subscriptions: number
  total_revenue_kopecks: number
  pending_rewards: number
  open_tickets: number
  high_risk_users: number
}

// ─── Request / Response shapes ────────────────────────────────────────────────

export interface LoginRequest {
  username: string
  password: string
}

export interface RegisterRequest {
  username: string
  password: string
  referral_code?: string
  device_fingerprint?: string
}

export interface AuthResponse {
  token: string
  user_id: string
  referral_code?: string
}

export interface BuySubscriptionRequest {
  plan: SubscriptionPlan
  return_url?: string
}

export interface BuySubscriptionResponse {
  payment_id: string
  redirect_url: string
  amount_rub: number
  plan: SubscriptionPlan
  expires_in?: string
}

export interface BalanceResponse {
  yad_balance: number
  yad_ruble_value: number
  yad_to_kopecks: number
}

export interface ReferralsResponse {
  referral_link: string
  referral_count: number
  referrals: Referral[]
}

export interface CreatePromoRequest {
  code: string
  yad_amount: number
  max_uses: number
  expires_at?: string
}

export interface CreateTicketRequest {
  subject: string
  message: string
}

export interface ReplyTicketRequest {
  message: string
}

export interface UsePromoRequest {
  code: string
}

export interface BuyShopItemRequest {
  item_id: string
}

export interface SetRiskRequest {
  score: number
}

export interface ApiError {
  error: string
}
