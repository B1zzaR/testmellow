package domain

import (
	"time"

	"github.com/google/uuid"
)

// ─── Enums ────────────────────────────────────────────────────────────────────

type PaymentStatus string

const (
	PaymentStatusPending      PaymentStatus = "PENDING"
	PaymentStatusConfirmed    PaymentStatus = "CONFIRMED"
	PaymentStatusCanceled     PaymentStatus = "CANCELED"
	PaymentStatusChargebacked PaymentStatus = "CHARGEBACKED"
	PaymentStatusExpired      PaymentStatus = "EXPIRED"
)

// IsValidPaymentStatus returns true for statuses the system recognises.
// Used to reject unexpected values returned by the payment gateway so
// they are never stored in the database.
func IsValidPaymentStatus(s PaymentStatus) bool {
	switch s {
	case PaymentStatusPending, PaymentStatusConfirmed, PaymentStatusCanceled,
		PaymentStatusChargebacked, PaymentStatusExpired:
		return true
	}
	return false
}

type SubscriptionStatus string

const (
	SubStatusActive   SubscriptionStatus = "active"
	SubStatusExpired  SubscriptionStatus = "expired"
	SubStatusTrial    SubscriptionStatus = "trial"
	SubStatusCanceled SubscriptionStatus = "canceled"
	// SubStatusReverted marks a subscription whose underlying payment was
	// charged back / cancelled by the gateway after activation.
	SubStatusReverted SubscriptionStatus = "reverted"
)

type YADTxType string

const (
	YADTxReferralReward     YADTxType = "referral_reward"
	YADTxBonus              YADTxType = "bonus"
	YADTxSpent              YADTxType = "spent"
	YADTxPromo              YADTxType = "promo"
	YADTxTrial              YADTxType = "trial"
	YADTxChargebackClawback YADTxType = "chargeback_clawback"
	// YADTxAdminAdjust marks balance changes performed manually by an admin.
	// Carrying it as a distinct tx_type lets ledger queries separate genuine
	// bonuses from administrative corrections — counting negative `bonus`
	// rows as "bonuses given out" was misleading.
	YADTxAdminAdjust YADTxType = "admin_adjust"
)

type TicketStatus string

const (
	TicketOpen     TicketStatus = "open"
	TicketAnswered TicketStatus = "answered"
	TicketClosed   TicketStatus = "closed"
)

type RewardSplitStatus string

const (
	SplitPending   RewardSplitStatus = "pending"
	SplitImmediate RewardSplitStatus = "immediate"
	SplitDeferred  RewardSplitStatus = "deferred"
	SplitPaid      RewardSplitStatus = "paid"
	SplitBlocked   RewardSplitStatus = "blocked"
	// SplitReverted is set when a referral reward is clawed back after a
	// chargeback on the originating payment.
	SplitReverted RewardSplitStatus = "reverted"
)

// ─── User ─────────────────────────────────────────────────────────────────────

type User struct {
	ID                    uuid.UUID  `db:"id" json:"id"`
	TelegramID            *int64     `db:"telegram_id" json:"telegram_id"`
	TelegramUsername      *string    `db:"telegram_username" json:"telegram_username"`
	TelegramFirstName     *string    `db:"telegram_first_name" json:"telegram_first_name"`
	TelegramLastName      *string    `db:"telegram_last_name" json:"telegram_last_name"`
	TelegramPhotoURL      *string    `db:"telegram_photo_url" json:"telegram_photo_url"`
	Username              *string    `db:"username" json:"username"`
	Email                 *string    `db:"email" json:"email"`
	PasswordHash          *string    `db:"password_hash" json:"-"`
	YADBalance            int64      `db:"yad_balance" json:"yad_balance"`
	ReferrerID            *uuid.UUID `db:"referrer_id" json:"referrer_id"`
	ReferralCode          string     `db:"referral_code" json:"referral_code"`
	LTV                   int64      `db:"ltv" json:"ltv_kopecks"`
	RiskScore             int        `db:"risk_score" json:"-"`
	IsAdmin               bool       `db:"is_admin" json:"is_admin"`
	IsBanned              bool       `db:"is_banned" json:"is_banned"`
	RemnaUserUUID         *string    `db:"remna_user_uuid" json:"-"`
	DeviceFingerprint     *string    `db:"device_fingerprint" json:"-"`
	LastKnownIP           *string    `db:"last_known_ip" json:"-"`
	TrialUsed             bool       `db:"trial_used" json:"trial_used"`
	ActiveDiscountCode    *string    `db:"active_discount_code" json:"active_discount_code,omitempty"`
	ActiveDiscountPercent int        `db:"active_discount_percent" json:"active_discount_percent"`
	TFAEnabled            bool       `db:"tfa_enabled" json:"tfa_enabled"`
	CreatedAt             time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at" json:"updated_at"`
}

// RemnaUsername returns the identifier to use as the Remnawave username.
// Prefers the website login; falls back to the user UUID so every user
// has a deterministic, unique Remnawave name.
func (u *User) RemnaUsername() string {
	if u.Username != nil && *u.Username != "" {
		return *u.Username
	}
	return u.ID.String()
}

// ─── Subscription ─────────────────────────────────────────────────────────────

type SubscriptionPlan string

const (
	PlanWeek       SubscriptionPlan = "1week"
	PlanMonth      SubscriptionPlan = "1month"
	PlanThreeMonth SubscriptionPlan = "3months"
	Plan99Years    SubscriptionPlan = "99years"
)

// PlanPriceKopecks returns price in kopecks (rubles × 100)
func PlanPriceKopecks(plan SubscriptionPlan) int64 {
	switch plan {
	case PlanWeek:
		return 4000
	case PlanMonth:
		return 10000
	case PlanThreeMonth:
		return 27000
	case Plan99Years:
		return 0
	}
	return 0
}

// PlanDurationDays returns duration in days
func PlanDurationDays(plan SubscriptionPlan) int {
	switch plan {
	case PlanWeek:
		return 7
	case PlanMonth:
		return 30
	case PlanThreeMonth:
		return 90
	case Plan99Years:
		return 36135
	}
	return 0
}

// PlanYADBonus returns the ЯД bonus credited when a plan is purchased with rubles via payment.
func PlanYADBonus(plan SubscriptionPlan) int64 {
	switch plan {
	case PlanWeek:
		return 10
	case PlanMonth:
		return 25
	case PlanThreeMonth:
		return 75
	case Plan99Years:
		return 0
	}
	return 0
}

// PlanYADPrice returns the ЯД price for purchasing a plan directly using ЯД balance.
func PlanYADPrice(plan SubscriptionPlan) int64 {
	switch plan {
	case PlanWeek:
		return 30
	case PlanMonth:
		return 75
	case PlanThreeMonth:
		return 210
	case Plan99Years:
		return 0
	}
	return 0
}

type Subscription struct {
	ID           uuid.UUID          `db:"id" json:"id"`
	UserID       uuid.UUID          `db:"user_id" json:"user_id"`
	Plan         SubscriptionPlan   `db:"plan" json:"plan"`
	Status       SubscriptionStatus `db:"status" json:"status"`
	StartsAt     time.Time          `db:"starts_at" json:"starts_at"`
	ExpiresAt    time.Time          `db:"expires_at" json:"expires_at"`
	RemnaSubUUID *string            `db:"remna_sub_uuid" json:"-"`
	PaidKopecks  int64              `db:"paid_kopecks" json:"paid_kopecks"`
	PaymentID    *uuid.UUID         `db:"payment_id" json:"payment_id"`
	CreatedAt    time.Time          `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `db:"updated_at" json:"updated_at"`
	Username     *string            `json:"username,omitempty"`
}

// ─── Payment ──────────────────────────────────────────────────────────────────

type Payment struct {
	ID                uuid.UUID        `db:"id" json:"id"`
	UserID            uuid.UUID        `db:"user_id" json:"user_id"`
	AmountKopecks     int64            `db:"amount_kopecks" json:"amount_kopecks"`
	Currency          string           `db:"currency" json:"currency"`
	Status            PaymentStatus    `db:"status" json:"status"`
	Plan              SubscriptionPlan `db:"plan" json:"plan"`
	PaymentMethod     int              `db:"payment_method" json:"payment_method"`
	PlategaPayload    string           `db:"platega_payload" json:"-"`
	RedirectURL       string           `db:"redirect_url" json:"redirect_url"`
	AddonQty          int              `db:"addon_qty" json:"addon_qty,omitempty"`
	ExpiresAt         *time.Time       `db:"expires_at" json:"expires_at"`
	WebhookReceivedAt *time.Time       `db:"webhook_received_at" json:"-"`
	Idempotency       string           `db:"idempotency" json:"-"`
	CreatedAt         time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time        `db:"updated_at" json:"updated_at"`
	Username          *string          `json:"username,omitempty"`
}

// ─── Referral ─────────────────────────────────────────────────────────────────

type Referral struct {
	ID           uuid.UUID `db:"id" json:"id"`
	ReferrerID   uuid.UUID `db:"referrer_id" json:"referrer_id"`
	RefereeID    uuid.UUID `db:"referee_id" json:"referee_id"`
	TotalPaidLTV int64     `db:"total_paid_ltv" json:"total_paid_ltv"`
	TotalReward  int64     `db:"total_reward" json:"total_reward"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

type ReferralReward struct {
	ID           uuid.UUID         `db:"id" json:"id"`
	ReferralID   uuid.UUID         `db:"referral_id" json:"referral_id"`
	PaymentID    uuid.UUID         `db:"payment_id" json:"payment_id"`
	ReferrerID   uuid.UUID         `db:"referrer_id" json:"referrer_id"`
	AmountYAD    int64             `db:"amount_yad" json:"amount_yad"`
	ImmediateYAD int64             `db:"immediate_yad" json:"immediate_yad"`
	DeferredYAD  int64             `db:"deferred_yad" json:"deferred_yad"`
	Status       RewardSplitStatus `db:"status" json:"status"`
	RiskScore    int               `db:"risk_score" json:"-"`
	ScheduledAt  time.Time         `db:"scheduled_at" json:"scheduled_at"`
	DeferredAt   *time.Time        `db:"deferred_at" json:"deferred_at"`
	PaidAt       *time.Time        `db:"paid_at" json:"paid_at"`
	CreatedAt    time.Time         `db:"created_at" json:"created_at"`
}

// ─── YAD Transaction ──────────────────────────────────────────────────────────

type YADTransaction struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	UserID    uuid.UUID  `db:"user_id" json:"user_id"`
	Delta     int64      `db:"delta" json:"delta"`
	Balance   int64      `db:"balance" json:"balance"`
	TxType    YADTxType  `db:"tx_type" json:"tx_type"`
	RefID     *uuid.UUID `db:"ref_id" json:"ref_id"`
	Note      string     `db:"note" json:"note"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}

// ─── PromoCode ────────────────────────────────────────────────────────────────

const (
	PromoTypeYAD      = "yad"
	PromoTypeDiscount = "discount"
)

type PromoCode struct {
	ID              uuid.UUID  `db:"id" json:"id"`
	Code            string     `db:"code" json:"code"`
	PromoType       string     `db:"promo_type" json:"promo_type"`
	YADAmount       int64      `db:"yad_amount" json:"yad_amount"`
	DiscountPercent int        `db:"discount_percent" json:"discount_percent"`
	MaxUses         int        `db:"max_uses" json:"max_uses"`
	UsedCount       int        `db:"used_count" json:"used_count"`
	ExpiresAt       *time.Time `db:"expires_at" json:"expires_at"`
	CreatedByID     uuid.UUID  `db:"created_by_id" json:"created_by_id"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
}

type PromoCodeUse struct {
	ID          uuid.UUID `db:"id" json:"id"`
	PromoCodeID uuid.UUID `db:"promo_code_id" json:"promo_code_id"`
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	UsedAt      time.Time `db:"used_at" json:"used_at"`
}

// ─── Ticket ───────────────────────────────────────────────────────────────────

type Ticket struct {
	ID        uuid.UUID    `db:"id" json:"id"`
	UserID    uuid.UUID    `db:"user_id" json:"user_id"`
	Subject   string       `db:"subject" json:"subject"`
	Status    TicketStatus `db:"status" json:"status"`
	CreatedAt time.Time    `db:"created_at" json:"created_at"`
	UpdatedAt time.Time    `db:"updated_at" json:"updated_at"`
}

type TicketMessage struct {
	ID        uuid.UUID `db:"id" json:"id"`
	TicketID  uuid.UUID `db:"ticket_id" json:"ticket_id"`
	SenderID  uuid.UUID `db:"sender_id" json:"sender_id"`
	IsAdmin   bool      `db:"is_admin" json:"is_admin"`
	Body      string    `db:"body" json:"body"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ─── Shop ─────────────────────────────────────────────────────────────────────

type ShopItem struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description"`
	PriceYAD    int64     `db:"price_yad" json:"price_yad"`
	Stock       int       `db:"stock" json:"stock"`
	IsActive    bool      `db:"is_active" json:"is_active"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}

type ShopOrder struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	ItemID    uuid.UUID `db:"item_id" json:"item_id"`
	Quantity  int       `db:"quantity" json:"quantity"`
	TotalYAD  int64     `db:"total_yad" json:"total_yad"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ─── Admin Audit Log ──────────────────────────────────────────────────────────

type AdminAuditLog struct {
	ID            uuid.UUID  `db:"id"          json:"id"`
	AdminID       uuid.UUID  `db:"admin_id"    json:"admin_id"`
	Action        string     `db:"action"      json:"action"`
	TargetType    *string    `db:"target_type" json:"target_type,omitempty"`
	TargetID      *uuid.UUID `db:"target_id"   json:"target_id,omitempty"`
	Details       *string    `db:"details"     json:"details,omitempty"`
	AdminUsername *string    `db:"-"           json:"admin_username,omitempty"`
	AdminEmail    *string    `db:"-"           json:"admin_email,omitempty"`
	CreatedAt     time.Time  `db:"created_at"  json:"created_at"`
}

// ─── Device ───────────────────────────────────────────────────────────────────

const DeviceMaxPerUser = 4
const DeviceInactiveDays = 3

// ─── Device Expansion ─────────────────────────────────────────────────────────

const DeviceExpansionMaxExtra = 2
const DeviceMaxWithExpansion = DeviceMaxPerUser + DeviceExpansionMaxExtra // 6

const (
	PlanDeviceExpansion  SubscriptionPlan = "device_expansion"   // +1 device
	PlanDeviceExpansion2 SubscriptionPlan = "device_expansion_2" // +2 devices
)

// IsDeviceExpansionPlan reports whether p is a device expansion plan.
func IsDeviceExpansionPlan(p SubscriptionPlan) bool {
	return p == PlanDeviceExpansion || p == PlanDeviceExpansion2
}

// DeviceExpansionQuantity returns the number of extra device slots for a plan.
func DeviceExpansionQuantity(p SubscriptionPlan) int {
	if p == PlanDeviceExpansion2 {
		return 2
	}
	return 1
}

// DeviceExpansionUnitKopecks returns the per-slot kopeck price based on days remaining in the subscription.
//
//	> 30 days  → 10 000 к (100 ₽)  — стандарт
//	8–30 days  →  4 000 к ( 40 ₽)  — −60 %
//	< 8 days   →  2 000 к ( 20 ₽)  — −80 %
func DeviceExpansionUnitKopecks(daysRemaining int) int64 {
	switch {
	case daysRemaining > 30:
		return 10000
	case daysRemaining >= 8:
		return 4000
	default:
		return 2000
	}
}

// DeviceExpansionUnitYAD returns the per-slot YAD price based on days remaining.
func DeviceExpansionUnitYAD(daysRemaining int) int64 {
	switch {
	case daysRemaining > 30:
		return 50
	case daysRemaining >= 8:
		return 20
	default:
		return 10
	}
}

// DeviceExpansionKopecks returns the total kopeck price for qty extra slots.
// The second slot gets a 10% discount.
func DeviceExpansionKopecks(qty int, daysRemaining int) int64 {
	unit := DeviceExpansionUnitKopecks(daysRemaining)
	if qty == 2 {
		return unit + unit*9/10 // second slot: −10%
	}
	return unit
}

// DeviceExpansionYAD returns the total YAD price for qty extra slots.
// The second slot gets a 10% discount.
func DeviceExpansionYAD(qty int, daysRemaining int) int64 {
	unit := DeviceExpansionUnitYAD(daysRemaining)
	if qty == 2 {
		return unit + unit*9/10 // second slot: −10%
	}
	return unit
}

// DeviceExpansionTierLabel returns a human-readable tier discount label,
// or an empty string when the full price applies.
func DeviceExpansionTierLabel(daysRemaining int) string {
	switch {
	case daysRemaining > 30:
		return ""
	case daysRemaining >= 8:
		return "−60% от стандарта"
	default:
		return "−80% от стандарта"
	}
}

type DeviceExpansion struct {
	ID           uuid.UUID `db:"id"`
	UserID       uuid.UUID `db:"user_id"`
	ExtraDevices int       `db:"extra_devices"`
	ExpiresAt    time.Time `db:"expires_at"`
	CreatedAt    time.Time `db:"created_at"`
}

type Device struct {
	ID         uuid.UUID `db:"id"          json:"id"`
	UserID     uuid.UUID `db:"user_id"      json:"user_id"`
	DeviceName string    `db:"device_name" json:"device_name"`
	LastActive time.Time `db:"last_active" json:"last_active"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
	IsActive   bool      `db:"is_active"   json:"is_active"`
	HwidID     string    `db:"-"           json:"-"` // Remnawave HWID identifier (not persisted)
}

func (d *Device) IsInactive() bool {
	return time.Since(d.LastActive) > DeviceInactiveDays*24*time.Hour
}

// ─── Analytics helpers ────────────────────────────────────────────────────────

type RevenueStat struct {
	Date         time.Time `json:"date"`
	TotalKopecks int64     `json:"total_kopecks"`
	Count        int64     `json:"count"`
}

type TopReferrer struct {
	UserID         uuid.UUID `json:"user_id"`
	Username       *string   `json:"username"`
	Email          *string   `json:"email"`
	ReferralCount  int64     `json:"referral_count"`
	TotalRewardYAD int64     `json:"total_reward_yad"`
}

// ─── Platform Settings ────────────────────────────────────────────────────────

type PlatformSettings struct {
	ID                      int       `db:"id"                           json:"id"`
	BlockRealMoneyPurchases bool      `db:"block_real_money_purchases"   json:"block_real_money_purchases"`
	UpdatedAt               time.Time `db:"updated_at"                   json:"updated_at"`
}

// ─── System Notifications ─────────────────────────────────────────────────────

type NotificationType string

const (
	NotificationWarning NotificationType = "warning"
	NotificationError   NotificationType = "error"
	NotificationInfo    NotificationType = "info"
	NotificationSuccess NotificationType = "success"
)

type SystemNotification struct {
	ID        uuid.UUID        `db:"id"         json:"id"`
	Type      NotificationType `db:"type"       json:"type"`
	Title     string           `db:"title"      json:"title"`
	Message   string           `db:"message"    json:"message"`
	IsActive  bool             `db:"is_active"  json:"is_active"`
	CreatedBy *uuid.UUID       `db:"created_by" json:"created_by,omitempty"`
	CreatedAt time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt time.Time        `db:"updated_at" json:"updated_at"`
}

// ─── Account Activity ─────────────────────────────────────────────────────────

type AccountActivity struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	EventType string    `db:"event_type" json:"event_type"`
	IP        *string   `db:"ip" json:"ip"`
	UserAgent *string   `db:"user_agent" json:"user_agent"`
	Details   *string   `db:"details" json:"details"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ─── Suggestion ───────────────────────────────────────────────────────────────

type SuggestionStatus string

const (
	SuggestionNew      SuggestionStatus = "new"
	SuggestionRead     SuggestionStatus = "read"
	SuggestionArchived SuggestionStatus = "archived"
)

// Suggestion is an anonymous feedback item submitted by any authenticated user.
// No user identity is stored — only the body text and creation time.
type Suggestion struct {
	ID        uuid.UUID        `db:"id"         json:"id"`
	Body      string           `db:"body"       json:"body"`
	Status    SuggestionStatus `db:"status"     json:"status"`
	CreatedAt time.Time        `db:"created_at" json:"created_at"`
}
