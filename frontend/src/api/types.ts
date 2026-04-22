// ─── Domain types matching backend domain models ──────────────────────────────

export type PaymentStatus = 'PENDING' | 'CONFIRMED' | 'CANCELED' | 'CHARGEBACKED' | 'EXPIRED'
export type SubscriptionStatus = 'active' | 'expired' | 'trial' | 'canceled'
export type SubscriptionPlan = '1week' | '1month' | '3months' | '99years';
export type YADTxType = 'referral_reward' | 'bonus' | 'spent' | 'promo' | 'trial'
export type TicketStatus = 'open' | 'answered' | 'closed'
export type RewardSplitStatus = 'pending' | 'immediate' | 'deferred' | 'paid' | 'blocked'

export interface User {
  id: string
  username: string | null
  email?: string | null
  telegram_id: number | null
  telegram_username: string | null
  telegram_first_name: string | null
  telegram_last_name: string | null
  telegram_photo_url: string | null
  yad_balance: number
  referral_code: string
  ltv: number
  trial_used: boolean
  is_admin: boolean
  is_banned: boolean
  tfa_enabled: boolean
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
  username?: string | null
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
  username?: string | null
}

export interface Referral {
  id: string
  referrer_id: string
  referee_id: string
  total_paid_ltv: number
  total_reward: number
  created_at: string
}

export interface TrafficStats {
  used_bytes: number
  limit_bytes: number
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

export type PromoType = 'yad' | 'discount'

export interface PromoCode {
  id: string
  code: string
  promo_type: PromoType
  yad_amount: number
  discount_percent: number
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

export interface Device {
  id: string
  device_name: string
  last_active: string
  is_active: boolean
  is_inactive: boolean
  can_delete_after: string
  is_blocked: boolean
}

export interface DeviceListResponse {
  devices: Device[]
  count: number
  limit: number
  expansion: DeviceExpansion | null
}

export interface DeviceExpansion {
  id: string
  user_id: string
  extra_devices: number
  expires_at: string
  created_at: string
}

export interface AccountActivity {
  id: string
  user_id: string
  event_type: string
  ip: string | null
  user_agent: string | null
  details: string | null
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
  user_id: string
  is_admin: boolean
  referral_code?: string
  tfa_required?: boolean
  challenge_id?: string
}

export interface TFACheckResponse {
  status: 'pending' | 'approved' | 'denied' | 'expired'
  user_id?: string
  is_admin?: boolean
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
  referral_code: string
  referral_count: number
  referrals: Referral[]
}

export interface CreatePromoRequest {
  code: string
  promo_type: PromoType
  yad_amount: number
  discount_percent: number
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

// ─── Admin extended types ─────────────────────────────────────────────────────

export interface AdminAuditLog {
  id: string
  admin_id: string
  action: string
  target_type?: string
  target_id?: string
  details?: string
  admin_username?: string
  admin_email?: string
  created_at: string
}

export interface RevenueStat {
  date: string
  total_kopecks: number
  count: number
}

export interface TopReferrer {
  user_id: string
  username: string | null
  email: string | null
  referral_count: number
  total_reward_yad: number
}

export interface RevenueAnalytics {
  revenue_by_day: RevenueStat[]
  top_referrers: TopReferrer[]
  period_days: number
}

export interface AdjustYADRequest {
  delta: number
  note: string
}

export interface AdminAdjustYADRequest {
  user_id: string
  delta: number
  note: string
}

export interface SetSubscriptionStatusRequest {
  status: SubscriptionStatus
}

export interface ExtendSubscriptionRequest {
  days: number
}

export interface CheckPaymentStatusResponse {
  payment_id: string
  platega_status: string
  db_status: string
}

export interface PlatformSettings {
  id: number
  block_real_money_purchases: boolean
  updated_at: string
}

export type NotificationType = 'warning' | 'error' | 'info' | 'success'

export interface SystemNotification {
  id: string
  type: NotificationType
  title: string
  message: string
  is_active: boolean
  created_by: string
  created_at: string
  updated_at: string
}

export interface CreateNotificationRequest {
  type: NotificationType
  title: string
  message: string
}

export interface UpdateNotificationRequest {
  type?: NotificationType
  title?: string
  message?: string
  is_active?: boolean
}

// ─── Suggestions ──────────────────────────────────────────────────────────────

export type SuggestionStatus = 'new' | 'read' | 'archived'

export interface Suggestion {
  id: string
  body: string
  status: SuggestionStatus
  created_at: string
}

export interface BroadcastRequest {
  message: string
}

export interface BroadcastResponse {
  queued: number
  total: number
}
