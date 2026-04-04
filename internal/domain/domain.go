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

type SubscriptionStatus string

const (
	SubStatusActive   SubscriptionStatus = "active"
	SubStatusExpired  SubscriptionStatus = "expired"
	SubStatusTrial    SubscriptionStatus = "trial"
	SubStatusCanceled SubscriptionStatus = "canceled"
)

type YADTxType string

const (
	YADTxReferralReward YADTxType = "referral_reward"
	YADTxBonus          YADTxType = "bonus"
	YADTxSpent          YADTxType = "spent"
	YADTxPromo          YADTxType = "promo"
	YADTxTrial          YADTxType = "trial"
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
)

// ─── User ─────────────────────────────────────────────────────────────────────

type User struct {
	ID                uuid.UUID  `db:"id" json:"id"`
	TelegramID        *int64     `db:"telegram_id" json:"telegram_id"`
	Username          *string    `db:"username" json:"username"`
	Email             *string    `db:"email" json:"email"`
	PasswordHash      *string    `db:"password_hash" json:"-"`
	YADBalance        int64      `db:"yad_balance" json:"yad_balance"`
	ReferrerID        *uuid.UUID `db:"referrer_id" json:"referrer_id"`
	ReferralCode      string     `db:"referral_code" json:"referral_code"`
	LTV               int64      `db:"ltv" json:"ltv"`
	RiskScore         int        `db:"risk_score" json:"-"`
	IsAdmin           bool       `db:"is_admin" json:"is_admin"`
	IsBanned          bool       `db:"is_banned" json:"is_banned"`
	RemnaUserUUID     *string    `db:"remna_user_uuid" json:"-"`
	DeviceFingerprint *string    `db:"device_fingerprint" json:"-"`
	LastKnownIP       *string    `db:"last_known_ip" json:"-"`
	TrialUsed         bool       `db:"trial_used" json:"trial_used"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

// ─── Subscription ─────────────────────────────────────────────────────────────

type SubscriptionPlan string

const (
	PlanWeek       SubscriptionPlan = "1week"
	PlanMonth      SubscriptionPlan = "1month"
	PlanThreeMonth SubscriptionPlan = "3months"
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
	ExpiresAt         *time.Time       `db:"expires_at" json:"expires_at"`
	WebhookReceivedAt *time.Time       `db:"webhook_received_at" json:"-"`
	Idempotency       string           `db:"idempotency" json:"-"`
	CreatedAt         time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time        `db:"updated_at" json:"updated_at"`
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

type PromoCode struct {
	ID          uuid.UUID  `db:"id" json:"id"`
	Code        string     `db:"code" json:"code"`
	YADAmount   int64      `db:"yad_amount" json:"yad_amount"`
	MaxUses     int        `db:"max_uses" json:"max_uses"`
	UsedCount   int        `db:"used_count" json:"used_count"`
	ExpiresAt   *time.Time `db:"expires_at" json:"expires_at"`
	CreatedByID uuid.UUID  `db:"created_by_id" json:"created_by_id"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
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
