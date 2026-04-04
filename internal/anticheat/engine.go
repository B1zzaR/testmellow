// Package anticheat implements the risk scoring, rate limiting, and abuse
// prevention systems for the VPN platform.
//
// Risk score: 0 = clean, 100 = maximum risk.
// Action limits, IP/fingerprint tracking, self-referral prevention, etc.
package anticheat

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	redisrepo "github.com/vpnplatform/internal/repository/redis"
)

// Configurable limits
const (
	MaxDailyYADCredit    = 5000 // max YAD a user can receive per day
	MaxDailyReferrals    = 10   // max new referrals per referrer per day
	MaxLoginAttempts     = 10   // brute-force threshold (15 min window)
	MaxPayoutsPerDay     = 20   // max reward payouts per user per day
	RiskScoreBlock       = 80   // block rewards above this score
	RiskScoreWarn        = 50   // reduce bonuses above this score
	BonusReductionFactor = 0.5  // multiply bonus by this if risk ≥ warn

	// Scoring increments
	DeltaSelfReferral    = 50
	DeltaSameIPReg       = 20
	DeltaSameFPReg       = 25
	DeltaRapidReferral   = 15
	DeltaSuspiciousPromo = 10
)

type Engine struct {
	rdb *redis.Client
	log *zap.Logger
}

func NewEngine(rdb *redis.Client, log *zap.Logger) *Engine {
	return &Engine{rdb: rdb, log: log}
}

// ─── Rate Limiting ────────────────────────────────────────────────────────────

// CheckLoginRateLimit returns error if login attempts exceed threshold (brute-force)
func (e *Engine) CheckLoginRateLimit(ctx context.Context, identifier string) error {
	count, err := redisrepo.RecordFailedLogin(ctx, e.rdb, identifier)
	if err != nil {
		return nil // fail open on Redis error, log separately
	}
	if count > MaxLoginAttempts {
		return fmt.Errorf("too many login attempts — locked for 15 minutes")
	}
	return nil
}

// ResetLoginAttempts clears the brute-force counter after a successful login
func (e *Engine) ResetLoginAttempts(ctx context.Context, identifier string) {
	_ = redisrepo.ResetFailedLogin(ctx, e.rdb, identifier)
}

// CheckAPIRateLimit is a generic per-user per-action rate limiter.
// Returns error if action count exceeds limit within the window.
func (e *Engine) CheckAPIRateLimit(ctx context.Context, userID, action string, limit int64, window time.Duration) error {
	key := fmt.Sprintf("rl:%s:%s", action, userID)
	count, err := redisrepo.Increment(ctx, e.rdb, key, window)
	if err != nil {
		return nil
	}
	if count > limit {
		return fmt.Errorf("rate limit exceeded for action %s", action)
	}
	return nil
}

// ResetRateLimit clears the rate-limit counter for a user + action so they can
// retry immediately. Used by admins to unblock a user.
func (e *Engine) ResetRateLimit(ctx context.Context, userID, action string) error {
	key := fmt.Sprintf("rl:%s:%s", action, userID)
	return e.rdb.Del(ctx, key).Err()
}

// CheckIPRateLimit rate-limits by IP for public endpoints
func (e *Engine) CheckIPRateLimit(ctx context.Context, ip, action string, limit int64, window time.Duration) error {
	key := fmt.Sprintf("rl:ip:%s:%s", action, ip)
	count, err := redisrepo.Increment(ctx, e.rdb, key, window)
	if err != nil {
		return nil
	}
	if count > limit {
		return fmt.Errorf("rate limit exceeded from IP %s", ip)
	}
	return nil
}

// ─── Referral Protection ─────────────────────────────────────────────────────

// CheckSelfReferral returns error if userID == referrerID
func (e *Engine) CheckSelfReferral(userID, referrerID uuid.UUID) error {
	if userID == referrerID {
		return fmt.Errorf("self-referral is not allowed")
	}
	return nil
}

// CheckDailyReferralLimit returns error if referrer has already registered
// MaxDailyReferrals today (from Redis counter)
func (e *Engine) CheckDailyReferralLimit(ctx context.Context, referrerID uuid.UUID) error {
	key := fmt.Sprintf("ref:daily:%s", referrerID.String())
	count, err := redisrepo.Increment(ctx, e.rdb, key, 24*time.Hour)
	if err != nil {
		return nil
	}
	if count > MaxDailyReferrals {
		// Remove the count we just added
		e.rdb.Decr(ctx, key)
		return fmt.Errorf("daily referral limit reached")
	}
	return nil
}

// ─── YAD Limits ───────────────────────────────────────────────────────────────

// CheckDailyYADCreditLimit returns error if user would exceed daily YAD cap.
// If allowed, adjusts the Redis daily counter.
func (e *Engine) CheckAndAddDailyYADCredit(ctx context.Context, userID uuid.UUID, amount int64) error {
	current, err := redisrepo.GetDailyYADCredit(ctx, e.rdb, userID.String())
	if err != nil {
		return nil // fail open
	}
	if current+amount > MaxDailyYADCredit {
		return fmt.Errorf("daily YAD credit limit exceeded")
	}
	return redisrepo.AddDailyYADCredit(ctx, e.rdb, userID.String(), amount)
}

// AdjustRewardForRisk reduces the reward amount based on risk score.
// Returns the adjusted amount.
func (e *Engine) AdjustRewardForRisk(originalYAD int64, riskScore int) int64 {
	if riskScore >= RiskScoreBlock {
		return 0
	}
	if riskScore >= RiskScoreWarn {
		return int64(float64(originalYAD) * BonusReductionFactor)
	}
	return originalYAD
}

// IsHighRisk returns true if risk score is above the block threshold.
func (e *Engine) IsHighRisk(riskScore int) bool {
	return riskScore >= RiskScoreBlock
}

// ─── Risk Score Helpers ───────────────────────────────────────────────────────

// ClampRiskScore ensures score stays within [0, 100]
func ClampRiskScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

// ─── Fingerprint / IP Checks ─────────────────────────────────────────────────

// RecordRegistrationAnomaly checks and records if a new registration IP or
// fingerprint is already associated with another user.
// Returns a risk delta to apply to the new user's score.
func (e *Engine) ScopeRegistrationRisk(ctx context.Context, ip, fingerprint string, sameIPCount, sameFPCount int) int {
	delta := 0
	if sameIPCount > 0 {
		e.log.Warn("new registration from known IP",
			zap.String("ip", ip),
			zap.Int("existing_users", sameIPCount),
		)
		delta += DeltaSameIPReg * min(sameIPCount, 3) // cap at 3x
	}
	if sameFPCount > 0 {
		e.log.Warn("new registration with known device fingerprint",
			zap.String("fingerprint", fingerprint),
			zap.Int("existing_users", sameFPCount),
		)
		delta += DeltaSameFPReg * min(sameFPCount, 3)
	}
	return ClampRiskScore(delta)
}

// RecordSuspiciousPromoUse adds risk for bulk promo code abuse
func (e *Engine) RecordSuspiciousPromoUse(ctx context.Context, userID uuid.UUID) int {
	return DeltaSuspiciousPromo
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── Deduplication ───────────────────────────────────────────────────────────

// EnsureOnce returns true if this is the first time the key is seen.
// Used for idempotent webhook processing at application level.
func (e *Engine) EnsureOnce(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return redisrepo.SetNX(ctx, e.rdb, "once:"+key, ttl)
}
