// Package worker implements all background job processing.
//
// Architecture:
//   - Redis lists as queues (LPUSH to enqueue, BRPOP to consume)
//   - All financial operations happen ONLY in the worker, never the API
//   - Idempotency checked before every operation
//
// Queue names:
//
//	payment:process     – process a confirmed Platega payment
//	subscription:activate – activate/extend Remnawave VPN access
//	referral:reward     – schedule LTV-based referral reward
//	referral:payout     – execute immediate or deferred payout
//	notify:telegram     – send Telegram notification
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/vpnplatform/internal/anticheat"
	"github.com/vpnplatform/internal/domain"
	"github.com/vpnplatform/internal/integration/platega"
	"github.com/vpnplatform/internal/integration/remnawave"
	"github.com/vpnplatform/internal/repository/postgres"
	redisrepo "github.com/vpnplatform/internal/repository/redis"
)

// isSerializationFailure reports whether err is PostgreSQL's
// serialization_failure (40001), raised when a Serializable transaction
// detects a write skew. Money-moving transactions retry on this error.
func isSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001"
	}
	return false
}

// ─── Queue name constants ───────────────────────────────────────────────────

const (
	QueuePaymentProcess             = "queue:payment:process"
	QueueSubscriptionActivate       = "queue:subscription:activate"
	QueueDeviceExpansionActivate    = "queue:device_expansion:activate"
	QueueReferralReward             = "queue:referral:reward"
	QueueReferralPayout             = "queue:referral:payout"
	QueueNotifyTelegram             = "queue:notify:telegram"
	QueueTFAChallenge               = "queue:tfa:challenge"
)

// ─── Job payload types ────────────────────────────────────────────────────────

type PaymentProcessJob struct {
	TransactionID string `json:"transaction_id"`
	UserID        string `json:"user_id"`
	AmountKopecks int64  `json:"amount_kopecks"`
	Plan          string `json:"plan"`
	Status        string `json:"status"`
}

type SubscriptionActivateJob struct {
	UserID        string `json:"user_id"`
	PaymentID     string `json:"payment_id"`
	Plan          string `json:"plan"`
	AmountKopecks int64  `json:"amount_kopecks"`
}

type DeviceExpansionActivateJob struct {
	UserID        string `json:"user_id"`
	PaymentID     string `json:"payment_id"`
	Qty           int    `json:"qty"`
	AmountKopecks int64  `json:"amount_kopecks"`
}

type ReferralRewardJob struct {
	PaymentID   string `json:"payment_id"`
	RefereeID   string `json:"referee_id"`
	PaidKopecks int64  `json:"paid_kopecks"`
}

type ReferralPayoutJob struct {
	RewardID   string `json:"reward_id"`
	IsDeferred bool   `json:"is_deferred"` // true = 70% payout
}

type NotifyTelegramJob struct {
	TelegramID int64  `json:"telegram_id"`
	Message    string `json:"message"`
}

type TFAChallengeJob struct {
	TelegramID  int64  `json:"telegram_id"`
	ChallengeID string `json:"challenge_id"`
	Message     string `json:"message"`
}

// ─── Enqueue helpers ─────────────────────────────────────────────────────────

func Enqueue(ctx context.Context, rdb *redis.Client, queue string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	return rdb.LPush(ctx, queue, string(data)).Err()
}

// ─── Worker ───────────────────────────────────────────────────────────────────

type Worker struct {
	rdb     *redis.Client
	repo    *postgres.UserRepo
	remna   *remnawave.Client
	platega *platega.Client
	anti    *anticheat.Engine
	tgToken string // Telegram bot token for notifications (may be empty)
	log     *zap.Logger
}

func NewWorker(
	rdb *redis.Client,
	repo *postgres.UserRepo,
	remna *remnawave.Client,
	platega *platega.Client,
	anti *anticheat.Engine,
	tgToken string,
	log *zap.Logger,
) *Worker {
	return &Worker{rdb: rdb, repo: repo, remna: remna, platega: platega, anti: anti, tgToken: tgToken, log: log}
}

// Run starts all worker goroutines and blocks.
func (w *Worker) Run(ctx context.Context) {
	go w.loop(ctx, QueuePaymentProcess, w.handlePaymentProcess)
	go w.loop(ctx, QueueSubscriptionActivate, w.handleSubscriptionActivate)
	go w.loop(ctx, QueueDeviceExpansionActivate, w.handleDeviceExpansionActivate)
	go w.loop(ctx, QueueReferralReward, w.handleReferralReward)
	go w.loop(ctx, QueueReferralPayout, w.handleReferralPayout)

	// Periodic tasks
	go w.periodicExpirySweep(ctx)
	go w.periodicRewardSweep(ctx)
	go w.periodicPaymentExpirySweep(ctx)
	go w.periodicExpiryWarnings(ctx)
	go w.periodicDeadQueueAlert(ctx)
	<-ctx.Done()
	w.log.Info("worker shutting down")
}

// periodicDeadQueueAlert logs (loudly) when any DLQ has accumulated jobs.
// The retry policy (requeueOrDead, max 5 attempts) silently moves failing
// jobs to "dead:<queue>" and otherwise forgets about them — without an
// alert path, payment activations stuck in the dead-letter queue can sit
// there for days while the operator wonders why a user's "paid" subscription
// never turned into VPN access. A periodic ERROR-level log is the minimum
// observability hook; pair this with an alert rule on the log shipper.
func (w *Worker) periodicDeadQueueAlert(ctx context.Context) {
	queues := []string{
		QueuePaymentProcess,
		QueueSubscriptionActivate,
		QueueDeviceExpansionActivate,
		QueueReferralReward,
		QueueReferralPayout,
		QueueNotifyTelegram,
		QueueTFAChallenge,
	}
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, q := range queues {
				n, err := w.rdb.LLen(ctx, "dead:"+q).Result()
				if err != nil {
					continue
				}
				if n > 0 {
					w.log.Error("dead-letter queue has stuck messages",
						zap.String("queue", q),
						zap.Int64("count", n))
				}
			}
		}
	}
}

// loop is the main BRPOP consume loop for a queue
func (w *Worker) loop(ctx context.Context, queue string, handler func(context.Context, string) error) {
	w.log.Info("worker loop started", zap.String("queue", queue))
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		result, err := w.rdb.BRPop(ctx, 5*time.Second, queue).Result()
		if err == redis.Nil {
			continue // timeout, retry
		}
		if err != nil {
			w.log.Error("brpop error", zap.String("queue", queue), zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		if len(result) < 2 {
			continue
		}
		payload := result[1]
		if err := handler(ctx, payload); err != nil {
			w.log.Error("job handler error",
				zap.String("queue", queue),
				zap.String("payload", payload),
				zap.Error(err),
			)
			w.requeueOrDead(ctx, queue, payload)
		}
	}
}

// ─── Payment Processing ───────────────────────────────────────────────────────

// requeueOrDead increments the _retries counter embedded in the JSON payload.
// If retries < maxJobRetries the job is pushed back onto the original queue;
// otherwise it is moved to "dead:<queue>" so an operator can inspect it.
const maxJobRetries = 5

func (w *Worker) requeueOrDead(ctx context.Context, queue, payload string) {
	var env map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &env); err != nil {
		// Not valid JSON — move straight to dead-letter.
		w.log.Warn("invalid job payload, moving to dead-letter", zap.String("queue", queue))
		_ = w.rdb.LPush(ctx, "dead:"+queue, payload).Err()
		return
	}

	var retries int
	if r, ok := env["_retries"]; ok {
		_ = json.Unmarshal(r, &retries)
	}

	retries++
	if retries > maxJobRetries {
		w.log.Warn("job exceeded max retries, moving to dead-letter",
			zap.String("queue", queue),
			zap.Int("retries", retries),
		)
		_ = w.rdb.LPush(ctx, "dead:"+queue, payload).Err()
		return
	}

	retriesJSON, _ := json.Marshal(retries)
	env["_retries"] = retriesJSON
	updated, err := json.Marshal(env)
	if err != nil {
		w.log.Error("marshal retried job", zap.Error(err))
		return
	}

	// Exponential backoff: 2^retries seconds (2s, 4s, 8s, 16s, 32s).
	// Run in a separate goroutine so the consumer loop is never blocked (M-5).
	backoff := time.Duration(1<<uint(retries)) * time.Second
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if pushErr := w.rdb.LPush(ctx, queue, string(updated)).Err(); pushErr != nil {
			w.log.Error("re-queue failed", zap.String("queue", queue), zap.Error(pushErr))
		}
	}()
}

// ─── Payment Processing (continued) ──────────────────────────────────────────

// allowedPaidPlans is the closed set of plans that may legitimately reach the
// worker for a paid (CONFIRMED) flow. 99years and any future free plan must
// never be activated through a Platega payment, so we reject them up front
// instead of letting handleSubscriptionActivate silently grant 99 years of
// VPN if a malformed payment row sneaks in.
var allowedPaidPlans = map[domain.SubscriptionPlan]bool{
	domain.PlanWeek:              true,
	domain.PlanMonth:             true,
	domain.PlanThreeMonth:        true,
	domain.PlanDeviceExpansion:   true,
	domain.PlanDeviceExpansion2:  true,
}

func (w *Worker) handlePaymentProcess(ctx context.Context, payload string) error {
	var job PaymentProcessJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return fmt.Errorf("unmarshal payment job: %w", err)
	}

	txID, err := uuid.Parse(job.TransactionID)
	if err != nil {
		return fmt.Errorf("invalid transaction id: %w", err)
	}
	userID, err := uuid.Parse(job.UserID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}

	// Idempotency check (per status — CONFIRMED and CHARGEBACKED have separate keys).
	idempKey := fmt.Sprintf("pay:processed:%s:%s", job.TransactionID, job.Status)
	isNew, err := w.anti.EnsureOnce(ctx, idempKey, 48*time.Hour)
	if err != nil {
		return err
	}
	if !isNew {
		w.log.Info("duplicate payment event, skipping",
			zap.String("tx_id", job.TransactionID),
			zap.String("status", job.Status),
		)
		return nil
	}

	// Load payment — never trust the job amount, always read it from DB.
	payment, err := w.repo.GetPaymentByID(ctx, txID)
	if err != nil {
		return err
	}
	if payment == nil {
		return fmt.Errorf("payment %s not found", txID)
	}

	plan := domain.SubscriptionPlan(payment.Plan)
	status := domain.PaymentStatus(job.Status)

	switch status {
	case domain.PaymentStatusConfirmed:
		if !allowedPaidPlans[plan] {
			w.log.Error("payment confirmed for non-paid plan — refusing to activate",
				zap.String("tx_id", txID.String()),
				zap.String("plan", string(plan)))
			// Mark the row CONFIRMED so the user sees their money landed but
			// abort activation. Operator will reconcile manually.
			tx, _ := w.repo.BeginSerializableTx(ctx)
			if tx != nil {
				_ = w.repo.UpdatePaymentStatus(ctx, tx, txID, status)
				_ = tx.Commit(ctx)
			}
			return nil
		}
		return w.processConfirmedPayment(ctx, txID, userID, payment)
	case domain.PaymentStatusCanceled, domain.PaymentStatusChargebacked:
		return w.processRevertedPayment(ctx, txID, userID, payment, status)
	default:
		// Unknown status — just persist it and stop.
		tx, err := w.repo.BeginSerializableTx(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if err := w.repo.UpdatePaymentStatus(ctx, tx, txID, status); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
}

// processConfirmedPayment is the hot path: payment status -> CONFIRMED,
// LTV += amount, then enqueue activation + referral reward.
func (w *Worker) processConfirmedPayment(ctx context.Context, txID, userID uuid.UUID,
	payment *domain.Payment,
) error {
	tx, err := w.repo.BeginSerializableTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := w.repo.LockUserForUpdate(ctx, tx, userID); err != nil {
		return err
	}
	if err := w.repo.UpdatePaymentStatus(ctx, tx, txID, domain.PaymentStatusConfirmed); err != nil {
		return err
	}
	// LTV uses the canonical DB amount, NOT a value from the webhook payload.
	if err := w.repo.UpdateLTV(ctx, tx, userID, payment.AmountKopecks); err != nil {
		return fmt.Errorf("update ltv: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	plan := domain.SubscriptionPlan(payment.Plan)
	if domain.IsDeviceExpansionPlan(plan) {
		qty := 1
		if payment.AddonQty > 0 {
			qty = payment.AddonQty
		}
		expansionJob := DeviceExpansionActivateJob{
			UserID:        userID.String(),
			PaymentID:     txID.String(),
			Qty:           qty,
			AmountKopecks: payment.AmountKopecks,
		}
		if err := Enqueue(ctx, w.rdb, QueueDeviceExpansionActivate, expansionJob); err != nil {
			w.log.Error("failed to enqueue device expansion activation", zap.Error(err))
		}
	} else {
		activateJob := SubscriptionActivateJob{
			UserID:        userID.String(),
			PaymentID:     txID.String(),
			Plan:          string(plan),
			AmountKopecks: payment.AmountKopecks,
		}
		if err := Enqueue(ctx, w.rdb, QueueSubscriptionActivate, activateJob); err != nil {
			w.log.Error("failed to enqueue subscription activation", zap.Error(err))
		}
	}

	// Referral reward only for non-zero paid plans. A free activation (100%
	// promo) used to mint 1 YAD here through the totalYAD-floor — that's a
	// vector for sign-up farming. Skip it cleanly.
	if payment.AmountKopecks > 0 {
		rewardJob := ReferralRewardJob{
			PaymentID:   txID.String(),
			RefereeID:   userID.String(),
			PaidKopecks: payment.AmountKopecks,
		}
		if err := Enqueue(ctx, w.rdb, QueueReferralReward, rewardJob); err != nil {
			w.log.Error("failed to enqueue referral reward", zap.Error(err))
		}
	}

	w.log.Info("payment confirmed",
		zap.String("tx_id", txID.String()),
		zap.Int64("kopecks", payment.AmountKopecks),
	)
	return nil
}

// processRevertedPayment is the chargeback / cancel compensation path. It
// is best-effort but transactional: every reversal step is idempotent and
// the function never claims success for a partially reverted payment.
//
// Compensations performed:
//   1. Payment row -> CANCELED / CHARGEBACKED.
//   2. LTV  -> -amount_kopecks (the additive UpdateLTV accepts negatives).
//   3. Subscription rolled back by PlanDurationDays(plan) days; if the new
//      expires_at is in the past, status flips to 'reverted' (a periodic
//      sweep then disables Remnawave). Expansion plans drop the row outright
//      and reset Remnawave's HwidDeviceLimit to the base.
//   4. Referral reward (if any) status -> 'reverted', YAD clawed back from
//      the referrer up to the available balance.
//
// Reverting a payment that was never CONFIRMED is a no-op for the LTV and
// subscription paths because `payment.WebhookReceivedAt` is nil and the
// DB-level state is already PENDING/EXPIRED.
func (w *Worker) processRevertedPayment(ctx context.Context, txID, userID uuid.UUID,
	payment *domain.Payment, newStatus domain.PaymentStatus,
) error {
	wasConfirmed := payment.Status == domain.PaymentStatusConfirmed
	plan := domain.SubscriptionPlan(payment.Plan)

	tx, err := w.repo.BeginSerializableTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := w.repo.LockUserForUpdate(ctx, tx, userID); err != nil {
		return err
	}
	if err := w.repo.UpdatePaymentStatus(ctx, tx, txID, newStatus); err != nil {
		return err
	}

	// Only reverse accounting for payments that had been previously confirmed.
	// PENDING -> CHARGEBACKED has nothing to undo.
	revertedExpiry := time.Time{}
	subNowExpired := false
	if wasConfirmed {
		// LTV: subtract the same amount we previously added.
		if err := w.repo.UpdateLTV(ctx, tx, userID, -payment.AmountKopecks); err != nil {
			return fmt.Errorf("revert ltv: %w", err)
		}

		// Subscription / expansion reversal.
		if domain.IsDeviceExpansionPlan(plan) {
			if err := w.repo.DeleteDeviceExpansionForUser(ctx, tx, userID); err != nil {
				return fmt.Errorf("revert device expansion: %w", err)
			}
		} else {
			sub, err := w.repo.LockLatestSubscriptionForUpdate(ctx, tx, userID)
			if err != nil {
				return fmt.Errorf("lock sub for revert: %w", err)
			}
			if sub != nil {
				days := domain.PlanDurationDays(plan)
				if days > 0 {
					exp, expired, err := w.repo.ContractSubscriptionByDuration(ctx, tx, sub.ID, days)
					if err != nil {
						return fmt.Errorf("contract sub: %w", err)
					}
					revertedExpiry = exp
					subNowExpired = expired
				}
			}
		}

		// Referral reward clawback (best-effort, clamped at zero balance).
		if reward, err := w.repo.GetRewardByPaymentID(ctx, txID); err == nil && reward != nil {
			reverted, err := w.repo.RevertReferralReward(ctx, tx, reward.ID)
			if err != nil {
				return fmt.Errorf("revert reward: %w", err)
			}
			if reverted && reward.ImmediateYAD > 0 {
				refID := reward.ID
				clawed, err := w.repo.ClawbackYAD(ctx, tx, reward.ReferrerID,
					reward.ImmediateYAD, &refID,
					fmt.Sprintf("Chargeback clawback for payment %s", txID))
				if err != nil {
					return fmt.Errorf("clawback yad: %w", err)
				}
				w.log.Info("referral reward clawed back",
					zap.String("referrer_id", reward.ReferrerID.String()),
					zap.Int64("requested", reward.ImmediateYAD),
					zap.Int64("clawed", clawed))
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// Post-commit: turn off VPN access if the chargeback retired the sub.
	user, _ := w.repo.GetByID(ctx, userID)
	if subNowExpired && user != nil && user.RemnaUserUUID != nil && *user.RemnaUserUUID != "" {
		if err := w.remna.DisableUser(ctx, *user.RemnaUserUUID); err != nil {
			w.log.Error("disable remnawave user after chargeback",
				zap.String("user_id", userID.String()), zap.Error(err))
		}
		_ = w.remna.UpdateHwidDeviceLimit(ctx, *user.RemnaUserUUID, domain.DeviceMaxPerUser)
	}
	if wasConfirmed && domain.IsDeviceExpansionPlan(plan) && user != nil &&
		user.RemnaUserUUID != nil && *user.RemnaUserUUID != "" {
		_ = w.remna.UpdateHwidDeviceLimit(ctx, *user.RemnaUserUUID, domain.DeviceMaxPerUser)
	}

	w.log.Info("payment reverted",
		zap.String("tx_id", txID.String()),
		zap.String("status", string(newStatus)),
		zap.Bool("was_confirmed", wasConfirmed),
		zap.Time("new_expiry", revertedExpiry),
	)
	return nil
}

// ─── Subscription Activation (Remnawave) ─────────────────────────────────────

func (w *Worker) handleSubscriptionActivate(ctx context.Context, payload string) error {
	var job SubscriptionActivateJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return err
	}

	userID, err := uuid.Parse(job.UserID)
	if err != nil {
		return fmt.Errorf("invalid user id %q: %w", job.UserID, err)
	}
	paymentID, err := uuid.Parse(job.PaymentID)
	if err != nil {
		return fmt.Errorf("invalid payment id %q: %w", job.PaymentID, err)
	}
	plan := domain.SubscriptionPlan(job.Plan)

	user, err := w.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return fmt.Errorf("user %s not found", userID)
	}

	now := time.Now()
	durationDays := domain.PlanDurationDays(plan)
	if durationDays == 0 {
		return fmt.Errorf("unknown plan: %s", plan)
	}
	if !allowedPaidPlans[plan] {
		return fmt.Errorf("plan %q is not allowed in paid activation flow", plan)
	}

	// Compute the post-update expiry inside a Serializable transaction with
	// the user row + latest sub row locked. The relative-update SQL prevents
	// two concurrent CONFIRMED webhooks from both reading the same
	// expires_at and both writing the same target — that race used to
	// silently lose one extension.
	var newExpiry time.Time
	for attempt := 0; attempt < 3; attempt++ {
		var err error
		newExpiry, err = w.extendOrCreateSub(ctx, userID, paymentID, plan, durationDays, job.AmountKopecks, now)
		if err == nil {
			break
		}
		if isSerializationFailure(err) && attempt < 2 {
			w.log.Info("subscription activate serialisation retry",
				zap.String("user_id", userID.String()), zap.Int("attempt", attempt))
			continue
		}
		return err
	}

	// Post-commit Remnawave sync. Failure here is recoverable manually; the
	// DB record already exists so the user keeps their paid time.
	var remnaUUID string
	if user.RemnaUserUUID == nil || *user.RemnaUserUUID == "" {
		remnaName := user.RemnaUsername()
		remnaUser, err := w.remna.CreateUser(ctx, remnaName, newExpiry)
		if err != nil {
			existing, lookupErr := w.remna.GetUserByUsername(ctx, remnaName)
			if lookupErr != nil || existing == nil {
				existing, lookupErr = w.remna.GetUserByUsername(ctx, userID.String())
			}
			if lookupErr != nil || existing == nil {
				return fmt.Errorf("create remna user: %w", err)
			}
			w.log.Info("recovered existing remnawave user",
				zap.String("user_id", userID.String()),
				zap.String("remna_uuid", existing.UUID))
			remnaUser = existing
		}
		remnaUUID = remnaUser.UUID
		if err := w.repo.UpdateRemnaUUID(ctx, userID, remnaUUID); err != nil {
			w.log.Error("update remna uuid", zap.Error(err))
		}
		_ = w.remna.UpdateExpiry(ctx, remnaUUID, newExpiry)
		_ = w.remna.EnableUser(ctx, remnaUUID)
	} else {
		remnaUUID = *user.RemnaUserUUID
		if err := w.remna.UpdateExpiry(ctx, remnaUUID, newExpiry); err != nil {
			return fmt.Errorf("update remna expiry: %w", err)
		}
		if err := w.remna.EnableUser(ctx, remnaUUID); err != nil {
			w.log.Warn("enable remna user failed", zap.Error(err))
		}
	}

	w.log.Info("subscription activated",
		zap.String("user_id", userID.String()),
		zap.String("plan", string(plan)),
		zap.Time("expires_at", newExpiry),
	)

	// Notify via Telegram
	if user.TelegramID != nil {
		planRu := map[domain.SubscriptionPlan]string{
			domain.PlanWeek:       "1 неделя",
			domain.PlanMonth:      "1 месяц",
			domain.PlanThreeMonth: "3 месяца",
		}
		label := planRu[plan]
		if label == "" {
			label = string(plan)
		}
		w.enqueueNotify(ctx, *user.TelegramID,
			fmt.Sprintf("✅ <b>Подписка активирована!</b>\n\nТариф: %s\nДействует до: %s", label, newExpiry.Format("02.01.2006")),
		)
	}

	// Credit ЯД bonus for purchasing with rubles
	if bonus := domain.PlanYADBonus(plan); bonus > 0 {
		bonusTx, err := w.repo.BeginTx(ctx)
		if err == nil {
			if err := w.repo.AdjustYADBalance(ctx, bonusTx, userID, bonus, domain.YADTxBonus, nil, "Бонус за тариф: "+string(plan)); err != nil {
				bonusTx.Rollback(ctx)
				w.log.Warn("failed to credit ЯД subscription bonus",
					zap.String("user_id", userID.String()),
					zap.Int64("bonus", bonus),
					zap.Error(err),
				)
			} else {
				_ = bonusTx.Commit(ctx)
			}
		}
	}

	return nil
}

// extendOrCreateSub is the per-attempt body of handleSubscriptionActivate.
// Inside a Serializable transaction it locks the user row, then either
// extends the latest subscription with relative SQL or creates a new one.
// Returns the post-update expires_at.
func (w *Worker) extendOrCreateSub(ctx context.Context, userID, paymentID uuid.UUID,
	plan domain.SubscriptionPlan, durationDays int, paidKopecks int64, now time.Time,
) (time.Time, error) {
	tx, err := w.repo.BeginSerializableTx(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer tx.Rollback(ctx)

	if err := w.repo.LockUserForUpdate(ctx, tx, userID); err != nil {
		return time.Time{}, err
	}
	existingSub, err := w.repo.LockLatestSubscriptionForUpdate(ctx, tx, userID)
	if err != nil {
		return time.Time{}, err
	}

	var newExpiry time.Time
	if existingSub != nil {
		newExpiry, err = w.repo.ExtendSubscriptionByDuration(ctx, tx, existingSub.ID, durationDays, plan)
		if err != nil {
			return time.Time{}, err
		}
	} else {
		pid := paymentID
		newExpiry = now.Add(time.Duration(durationDays) * 24 * time.Hour)
		sub := &domain.Subscription{
			ID:          uuid.New(),
			UserID:      userID,
			Plan:        plan,
			Status:      domain.SubStatusActive,
			StartsAt:    now,
			ExpiresAt:   newExpiry,
			PaidKopecks: paidKopecks,
			PaymentID:   &pid,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := w.repo.CreateSubscription(ctx, tx, sub); err != nil {
			return time.Time{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, err
	}
	return newExpiry, nil
}

// ─── Referral Reward ──────────────────────────────────────────────────────────
// LTV = total kopecks paid by the referee
// Reward = 15% of payment amount in YAD  (1 YAD = 2.5 ₽)
// 100% paid immediately (after 24–72h delay)

func (w *Worker) handleReferralReward(ctx context.Context, payload string) error {
	var job ReferralRewardJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return err
	}

	refereeID, err := uuid.Parse(job.RefereeID)
	if err != nil {
		return fmt.Errorf("invalid referee id %q: %w", job.RefereeID, err)
	}
	paymentID, err := uuid.Parse(job.PaymentID)
	if err != nil {
		return fmt.Errorf("invalid payment id %q: %w", job.PaymentID, err)
	}

	// Find referral
	referral, err := w.repo.GetReferralByReferee(ctx, refereeID)
	if err != nil {
		return err
	}
	if referral == nil {
		return nil // no referrer, nothing to do
	}

	// Load referrer to check risk score
	referrer, err := w.repo.GetByID(ctx, referral.ReferrerID)
	if err != nil || referrer == nil {
		return fmt.Errorf("referrer not found")
	}

	if w.anti.IsHighRisk(referrer.RiskScore) {
		w.log.Warn("skipping referral reward: high risk referrer",
			zap.String("referrer_id", referrer.ID.String()),
			zap.Int("risk_score", referrer.RiskScore),
		)
		return nil
	}

	// Free activations (100% promo) have no money behind them — never mint
	// a referral reward in that case. The previous "min 1 YAD" floor was a
	// sign-up farming vector when combined with multi-account abuse.
	if job.PaidKopecks <= 0 {
		return nil
	}

	// Calculate reward in pure integer arithmetic: 15% of payment in kopecks,
	// then 1 YAD per 250 kopecks. Floats here are unjustified and accumulate
	// rounding error; the integer form is exact and reproducible across
	// platforms.
	rewardKopecks := job.PaidKopecks * 15 / 100
	totalYAD := rewardKopecks / 250
	if totalYAD <= 0 {
		// Payment was real but smaller than 1 YAD's worth (≤ 1666 kopecks).
		// Round up to 1 YAD so micro-payments still produce a token reward.
		totalYAD = 1
	}

	// Apply risk-based adjustment
	totalYAD = w.anti.AdjustRewardForRisk(totalYAD, referrer.RiskScore)
	if totalYAD == 0 {
		w.log.Warn("reward zeroed by risk adjustment",
			zap.String("referrer_id", referrer.ID.String()))
		return nil
	}

	immediateYAD := totalYAD
	deferredYAD := int64(0)

	// No delay - pay immediately
	scheduledAt := time.Now()
	var deferredAt *time.Time

	rr := &domain.ReferralReward{
		ID:           uuid.New(),
		ReferralID:   referral.ID,
		PaymentID:    paymentID,
		ReferrerID:   referral.ReferrerID,
		AmountYAD:    totalYAD,
		ImmediateYAD: immediateYAD,
		DeferredYAD:  deferredYAD,
		Status:       domain.SplitPending,
		RiskScore:    referrer.RiskScore,
		ScheduledAt:  scheduledAt,
		DeferredAt:   deferredAt,
		CreatedAt:    time.Now(),
	}

	tx, err := w.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Create the reward record
	if err := w.repo.CreateReferralReward(ctx, tx, rr); err != nil {
		return err
	}
	// Update referral totals
	if err := w.repo.UpdateReferralTotals(ctx, tx, referral.ID, job.PaidKopecks, totalYAD); err != nil {
		return err
	}

	// Pay immediately without waiting
	refID := rr.ID
	if err := w.repo.AdjustYADBalance(ctx, tx, referral.ReferrerID,
		immediateYAD, domain.YADTxReferralReward, &refID,
		fmt.Sprintf("Referral reward (immediate)")); err != nil {
		return fmt.Errorf("credit immediate yad: %w", err)
	}
	if err := w.repo.UpdateRewardStatus(ctx, tx, rr.ID, domain.SplitImmediate); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	w.log.Info("referral reward paid immediately",
		zap.String("referrer_id", referral.ReferrerID.String()),
		zap.Int64("total_yad", totalYAD),
	)
	return nil
}

// ─── Referral Payout ─────────────────────────────────────────────────────────

func (w *Worker) handleReferralPayout(ctx context.Context, payload string) error {
	var job ReferralPayoutJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return err
	}

	rewardID, err := uuid.Parse(job.RewardID)
	if err != nil {
		return fmt.Errorf("invalid reward id %q: %w", job.RewardID, err)
	}

	// Use Redis lock to prevent double-payout
	lockKey := "reward:payout:" + rewardID.String()
	lockToken, locked, err := redisrepo.TryLock(ctx, w.rdb, lockKey, 5*time.Minute)
	if err != nil || !locked {
		w.log.Warn("payout lock contention, skipping", zap.String("reward_id", rewardID.String()))
		return nil
	}
	defer redisrepo.Unlock(ctx, w.rdb, lockKey, lockToken)

	tx, err := w.repo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Fetch the reward and credit it
	reward, err := w.repo.GetRewardByID(ctx, rewardID)
	if err != nil {
		return fmt.Errorf("get reward %s: %w", rewardID, err)
	}
	if reward == nil || reward.Status != domain.SplitPending {
		return nil // already processed or not found
	}

	refID := reward.ID
	if err := w.repo.AdjustYADBalance(ctx, tx, reward.ReferrerID,
		reward.ImmediateYAD, domain.YADTxReferralReward, &refID,
		"Referral reward (100% immediate)"); err != nil {
		return fmt.Errorf("credit yad: %w", err)
	}
	if err := w.repo.UpdateRewardStatus(ctx, tx, reward.ID, domain.SplitImmediate); err != nil {
		return fmt.Errorf("update reward status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	w.log.Info("referral reward paid via queue",
		zap.String("referrer_id", reward.ReferrerID.String()),
		zap.Int64("yad", reward.ImmediateYAD),
	)
	return nil
}

// ─── Periodic Sweeps ──────────────────────────────────────────────────────────

// periodicPaymentExpirySweep marks PENDING payments past their expires_at as
// EXPIRED. Before marking, it queries Platega for final status so that
// payments confirmed at the gateway but not yet webhoooked get activated.
func (w *Worker) periodicPaymentExpirySweep(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.syncStalePayments(ctx)
			n, err := w.repo.MarkExpiredPayments(ctx)
			if err != nil {
				w.log.Error("payment expiry sweep failed", zap.Error(err))
				continue
			}
			if n > 0 {
				w.log.Info("marked payments expired", zap.Int64("count", n))
			}
		}
	}
}

// syncStalePayments checks Platega for each nearly-expired PENDING payment and
// enqueues activation if confirmed, keeping the system correct even when the
// webhook was delayed or missed.
func (w *Worker) syncStalePayments(ctx context.Context) {
	payments, err := w.repo.GetStalePendingPayments(ctx)
	if err != nil {
		w.log.Error("get stale pending payments", zap.Error(err))
		return
	}
	for _, p := range payments {
		// Use Redis dedup key to avoid hammering Platega for the same payment.
		dedupKey := fmt.Sprintf("worker:sync:%s", p.ID.String())
		ok, _ := redisrepo.SetNX(ctx, w.rdb, dedupKey, 5*time.Minute)
		if !ok {
			continue
		}

		platResp, err := w.platega.GetPaymentStatus(ctx, p.ID.String())
		if err != nil {
			// Release the dedup key so the next periodic cycle can retry immediately.
			_ = w.rdb.Del(ctx, dedupKey)
			w.log.Warn("platega status sync failed",
				zap.String("payment_id", p.ID.String()),
				zap.Error(err))
			continue
		}

		newStatus := domain.PaymentStatus(platResp.Status)
		// Ignore unrecognised statuses — never store garbage in the DB.
		if !domain.IsValidPaymentStatus(newStatus) {
			w.log.Warn("unexpected payment status from Platega",
				zap.String("payment_id", p.ID.String()),
				zap.String("status", string(platResp.Status)))
			continue
		}
		if newStatus == p.Status {
			continue
		}

		_ = w.repo.UpdatePaymentStatus(ctx, nil, p.ID, newStatus)

		if newStatus == domain.PaymentStatusConfirmed {
			job := PaymentProcessJob{
				TransactionID: p.ID.String(),
				UserID:        p.UserID.String(),
				AmountKopecks: p.AmountKopecks,
				Plan:          string(p.Plan),
				Status:        string(newStatus),
			}
			if err := Enqueue(ctx, w.rdb, QueuePaymentProcess, job); err != nil {
				w.log.Error("enqueue payment from sync", zap.Error(err))
			}
			w.log.Info("payment confirmed via sync",
				zap.String("payment_id", p.ID.String()),
				zap.String("user_id", p.UserID.String()),
			)
		}
	}
}

// ─── Device Expansion Activation ─────────────────────────────────────────────

func (w *Worker) handleDeviceExpansionActivate(ctx context.Context, payload string) error {
	var job DeviceExpansionActivateJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return fmt.Errorf("unmarshal device expansion job: %w", err)
	}

	userID, err := uuid.Parse(job.UserID)
	if err != nil {
		return fmt.Errorf("parse user_id: %w", err)
	}
	paymentID, err := uuid.Parse(job.PaymentID)
	if err != nil {
		return fmt.Errorf("parse payment_id: %w", err)
	}

	// Idempotency check.
	idemKey := fmt.Sprintf("devexp:processed:%s", paymentID.String())
	set, err := w.rdb.SetNX(ctx, idemKey, "1", 48*time.Hour).Result()
	if err != nil {
		return fmt.Errorf("idempotency check: %w", err)
	}
	if !set {
		w.log.Info("device expansion already processed (idempotent skip)",
			zap.String("payment_id", paymentID.String()))
		return nil
	}

	qty := job.Qty
	if qty < 1 || qty > domain.DeviceExpansionMaxExtra {
		qty = 1
	}

	// Must have an active subscription.
	activeSub, err := w.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		return fmt.Errorf("get active subscription: %w", err)
	}
	if activeSub == nil {
		w.log.Warn("device expansion activate: no active subscription",
			zap.String("user_id", userID.String()))
		return nil
	}

	expansion := &domain.DeviceExpansion{
		ID:           paymentID, // reuse payment ID as expansion ID for traceability
		UserID:       userID,
		ExtraDevices: qty,
		ExpiresAt:    activeSub.ExpiresAt,
		CreatedAt:    time.Now(),
	}

	// Serializable + user lock: even though the SETNX idempotency key above
	// dedupes a single payment, two distinct paid expansions for the same
	// user must serialise on the user lock so the underlying UPSERT into
	// device_expansions(user_id) keeps both writes coherent.
	commit := func() error {
		tx, err := w.repo.BeginSerializableTx(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback(ctx)
		if err := w.repo.LockUserForUpdate(ctx, tx, userID); err != nil {
			return err
		}
		if err := w.repo.CreateDeviceExpansion(ctx, tx, expansion); err != nil {
			return fmt.Errorf("create device expansion: %w", err)
		}
		if err := w.repo.IncrementDeviceExpansionCount(ctx, tx, userID); err != nil {
			return fmt.Errorf("increment expansion count: %w", err)
		}
		return tx.Commit(ctx)
	}
	for attempt := 0; attempt < 3; attempt++ {
		if err := commit(); err != nil {
			if isSerializationFailure(err) && attempt < 2 {
				continue
			}
			return err
		}
		break
	}

	// Post-commit: update Remnawave device limit.
	user, err := w.repo.GetByID(ctx, userID)
	if err == nil && user != nil && user.RemnaUserUUID != nil && *user.RemnaUserUUID != "" {
		newLimit := domain.DeviceMaxPerUser + qty
		if err := w.remna.UpdateHwidDeviceLimit(ctx, *user.RemnaUserUUID, newLimit); err != nil {
			w.log.Error("handleDeviceExpansionActivate: update remnawave limit",
				zap.String("user_id", userID.String()),
				zap.Int("limit", newLimit),
				zap.Error(err))
		}
	}

	// Telegram notification.
	if user != nil && user.TelegramID != nil {
		msg := fmt.Sprintf("✅ Расширение устройств активировано!\n+%d слот(а) — до %s",
			qty, activeSub.ExpiresAt.Format("02.01.2006"))
		notifyJob := NotifyTelegramJob{TelegramID: *user.TelegramID, Message: msg}
		if err := Enqueue(ctx, w.rdb, QueueNotifyTelegram, notifyJob); err != nil {
			w.log.Error("enqueue telegram notify for expansion", zap.Error(err))
		}
	}

	w.log.Info("device expansion activated",
		zap.String("user_id", userID.String()),
		zap.Int("qty", qty),
		zap.String("expires_at", activeSub.ExpiresAt.Format(time.RFC3339)))
	return nil
}

// periodicExpirySweep marks expired subscriptions, disables VPN access, and resets device expansions
func (w *Worker) periodicExpirySweep(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Collect user IDs BEFORE marking expired so we know who to process.
			userIDs, err := w.repo.GetExpiredSubscriptionUserIDs(ctx)
			if err != nil {
				w.log.Error("get expired user ids", zap.Error(err))
			}

			n, err := w.repo.UpdateExpiredSubscriptions(ctx)
			if err != nil {
				w.log.Error("expiry sweep failed", zap.Error(err))
				continue
			}
			if n > 0 {
				w.log.Info("marked subscriptions expired", zap.Int64("count", n))
			}

			for _, uid := range userIDs {
				user, err := w.repo.GetByID(ctx, uid)
				if err != nil || user == nil {
					continue
				}

				// Disable Remnawave access
				if user.RemnaUserUUID != nil && *user.RemnaUserUUID != "" {
					if err := w.remna.DisableUser(ctx, *user.RemnaUserUUID); err != nil {
						w.log.Error("disable remnawave user", zap.String("uid", uid.String()), zap.Error(err))
					}
					// Reset device limit to base
					_ = w.remna.UpdateHwidDeviceLimit(ctx, *user.RemnaUserUUID, domain.DeviceMaxPerUser)
				}
			}

			// Clean up expired device expansions (may outlive subscription by edge cases).
			expiredUIDs, err := w.repo.DeleteExpiredDeviceExpansions(ctx)
			if err != nil {
				w.log.Error("delete expired device expansions", zap.Error(err))
			}
			for _, uid := range expiredUIDs {
				user, err := w.repo.GetByID(ctx, uid)
				if err != nil || user == nil || user.RemnaUserUUID == nil {
					continue
				}
				if err := w.remna.UpdateHwidDeviceLimit(ctx, *user.RemnaUserUUID, domain.DeviceMaxPerUser); err != nil {
					w.log.Error("reset device limit after expansion expiry",
						zap.String("uid", uid.String()), zap.Error(err))
				}
			}
		}
	}
}

// periodicRewardSweep pays out pending and deferred referral rewards
func (w *Worker) periodicRewardSweep(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.payoutPendingRewards(ctx)
			// w.payoutDeferredRewards(ctx)  // No longer needed: 100% immediate payout
		}
	}
}

func (w *Worker) payoutPendingRewards(ctx context.Context) {
	rewards, err := w.repo.GetPendingRewardsDue(ctx)
	if err != nil {
		w.log.Error("get pending rewards", zap.Error(err))
		return
	}
	for _, rr := range rewards {
		// Lock per reward
		lockKey := "reward:payout:imm:" + rr.ID.String()
		lockToken, locked, _ := redisrepo.TryLock(ctx, w.rdb, lockKey, 2*time.Minute)
		if !locked {
			continue
		}

		func() {
			defer redisrepo.Unlock(ctx, w.rdb, lockKey, lockToken)

			tx, err := w.repo.BeginTx(ctx)
			if err != nil {
				return
			}
			defer tx.Rollback(ctx)

			// Credit immediate 100%
			refID := rr.ID
			if err := w.repo.AdjustYADBalance(ctx, tx, rr.ReferrerID,
				rr.ImmediateYAD, domain.YADTxReferralReward, &refID,
				fmt.Sprintf("Referral reward (100%% immediate)")); err != nil {
				w.log.Error("credit immediate yad", zap.Error(err))
				return
			}
			if err := w.repo.UpdateRewardStatus(ctx, tx, rr.ID, domain.SplitImmediate); err != nil {
				return
			}
			if err := tx.Commit(ctx); err != nil {
				return
			}
			w.log.Info("immediate referral reward paid",
				zap.String("referrer_id", rr.ReferrerID.String()),
				zap.Int64("yad", rr.ImmediateYAD),
			)
		}()
	}
}

func (w *Worker) payoutDeferredRewards(ctx context.Context) {
	rewards, err := w.repo.GetDeferredRewardsDue(ctx)
	if err != nil {
		w.log.Error("get deferred rewards", zap.Error(err))
		return
	}
	for _, rr := range rewards {
		lockKey := "reward:payout:def:" + rr.ID.String()
		lockToken, locked, _ := redisrepo.TryLock(ctx, w.rdb, lockKey, 2*time.Minute)
		if !locked {
			continue
		}

		func() {
			defer redisrepo.Unlock(ctx, w.rdb, lockKey, lockToken)

			tx, err := w.repo.BeginTx(ctx)
			if err != nil {
				return
			}
			defer tx.Rollback(ctx)

			refID := rr.ID
			if err := w.repo.AdjustYADBalance(ctx, tx, rr.ReferrerID,
				rr.DeferredYAD, domain.YADTxReferralReward, &refID,
				fmt.Sprintf("Referral reward (deferred 70%%)")); err != nil {
				w.log.Error("credit deferred yad", zap.Error(err))
				return
			}
			if err := w.repo.UpdateRewardStatus(ctx, tx, rr.ID, domain.SplitPaid); err != nil {
				return
			}
			if err := tx.Commit(ctx); err != nil {
				return
			}
			w.log.Info("deferred referral reward paid",
				zap.String("referrer_id", rr.ReferrerID.String()),
				zap.Int64("yad", rr.DeferredYAD),
			)
		}()
	}
}

// ─── Telegram Notifications ───────────────────────────────────────────────────

// enqueueNotify enqueues a Telegram notification if the user has a telegram_id.
func (w *Worker) enqueueNotify(ctx context.Context, tgID int64, msg string) {
	if tgID == 0 || w.tgToken == "" {
		return
	}
	_ = Enqueue(ctx, w.rdb, QueueNotifyTelegram, NotifyTelegramJob{
		TelegramID: tgID,
		Message:    msg,
	})
}

// periodicExpiryWarnings sends Telegram notifications during the last 3 days before subscription expires.
// Days 2-3: one notification per 24 h. Last day (<24 h left): up to 3 notifications (~every 8 h).
func (w *Worker) periodicExpiryWarnings(ctx context.Context) {
	ticker := time.NewTicker(4 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sendExpiryWarnings(ctx)
		}
	}
}

func (w *Worker) sendExpiryWarnings(ctx context.Context) {
	if w.tgToken == "" {
		return
	}
	subs, err := w.repo.GetSubscriptionsExpiringIn(ctx, 3*24*time.Hour)
	if err != nil {
		w.log.Error("get expiring subscriptions", zap.Error(err))
		return
	}
	for _, sub := range subs {
		hoursLeft := time.Until(sub.ExpiresAt).Hours()
		daysLeft := int(hoursLeft / 24)

		// Determine dedup key and TTL based on time remaining:
		// Last day (<24h): dedup for 8h → up to 3 notifications per day
		// Days 2-3: dedup for 24h → 1 notification per day
		var dedupKey string
		var dedupTTL time.Duration
		if hoursLeft < 24 {
			slot := int(hoursLeft) / 8 // 0, 1, or 2
			dedupKey = fmt.Sprintf("notify:expiry:%s:d0:s%d", sub.ID.String(), slot)
			dedupTTL = 8 * time.Hour
		} else {
			dedupKey = fmt.Sprintf("notify:expiry:%s:d%d", sub.ID.String(), daysLeft)
			dedupTTL = 24 * time.Hour
		}

		ok, _ := redisrepo.SetNX(ctx, w.rdb, dedupKey, dedupTTL)
		if !ok {
			continue
		}
		user, err := w.repo.GetByID(ctx, sub.UserID)
		if err != nil || user == nil || user.TelegramID == nil {
			continue
		}

		var msg string
		if hoursLeft < 24 {
			h := int(hoursLeft)
			if h < 1 {
				h = 1
			}
			msg = fmt.Sprintf("🔴 <b>Ваша подписка истекает менее чем через %d ч!</b>\n\nСрочно продлите её в личном кабинете, чтобы не потерять доступ к VPN.", h)
		} else {
			msg = fmt.Sprintf("⚠️ <b>Ваша подписка истекает через %d дн.</b>\n\nПродлите её в личном кабинете, чтобы не потерять доступ к VPN.", daysLeft)
		}
		w.enqueueNotify(ctx, *user.TelegramID, msg)
	}
}

