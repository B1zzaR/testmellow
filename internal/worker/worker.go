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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/vpnplatform/internal/anticheat"
	"github.com/vpnplatform/internal/domain"
	"github.com/vpnplatform/internal/integration/platega"
	"github.com/vpnplatform/internal/integration/remnawave"
	"github.com/vpnplatform/internal/repository/postgres"
	redisrepo "github.com/vpnplatform/internal/repository/redis"
)

// ─── Queue name constants ───────────────────────────────────────────────────

const (
	QueuePaymentProcess       = "queue:payment:process"
	QueueSubscriptionActivate = "queue:subscription:activate"
	QueueReferralReward       = "queue:referral:reward"
	QueueReferralPayout       = "queue:referral:payout"
	QueueNotifyTelegram       = "queue:notify:telegram"
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
	go w.loop(ctx, QueueReferralReward, w.handleReferralReward)
	go w.loop(ctx, QueueReferralPayout, w.handleReferralPayout)
	go w.loop(ctx, QueueNotifyTelegram, w.handleNotifyTelegram)

	// Periodic tasks
	go w.periodicExpirySweep(ctx)
	go w.periodicRewardSweep(ctx)
	go w.periodicPaymentExpirySweep(ctx)
	go w.periodicExpiryWarnings(ctx)

	<-ctx.Done()
	w.log.Info("worker shutting down")
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

	// Exponential backoff: 2^retries seconds (2s, 4s, 8s, 16s, 32s)
	backoff := time.Duration(1<<uint(retries)) * time.Second
	select {
	case <-ctx.Done():
		return
	case <-time.After(backoff):
	}

	if pushErr := w.rdb.LPush(ctx, queue, string(updated)).Err(); pushErr != nil {
		w.log.Error("re-queue failed", zap.String("queue", queue), zap.Error(pushErr))
	}
}

// ─── Payment Processing (continued) ──────────────────────────────────────────

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

	// Idempotency check
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

	// Load payment
	payment, err := w.repo.GetPaymentByID(ctx, txID)
	if err != nil {
		return err
	}
	if payment == nil {
		return fmt.Errorf("payment %s not found", txID)
	}

	// Begin serializable transaction
	tx, err := w.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Update payment status
	if err := w.repo.UpdatePaymentStatus(ctx, tx, txID, domain.PaymentStatus(job.Status)); err != nil {
		return err
	}

	if domain.PaymentStatus(job.Status) == domain.PaymentStatusConfirmed {
		// Update user LTV
		if err := w.repo.UpdateLTV(ctx, tx, userID, job.AmountKopecks); err != nil {
			return fmt.Errorf("update ltv: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}

		// Enqueue subscription activation
		activateJob := SubscriptionActivateJob{
			UserID:        userID.String(),
			PaymentID:     txID.String(),
			Plan:          job.Plan,
			AmountKopecks: job.AmountKopecks,
		}
		if err := Enqueue(ctx, w.rdb, QueueSubscriptionActivate, activateJob); err != nil {
			w.log.Error("failed to enqueue subscription activation", zap.Error(err))
		}

		// Enqueue referral reward
		rewardJob := ReferralRewardJob{
			PaymentID:   txID.String(),
			RefereeID:   userID.String(),
			PaidKopecks: job.AmountKopecks,
		}
		if err := Enqueue(ctx, w.rdb, QueueReferralReward, rewardJob); err != nil {
			w.log.Error("failed to enqueue referral reward", zap.Error(err))
		}
	} else {
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}

	w.log.Info("payment processed",
		zap.String("tx_id", txID.String()),
		zap.String("status", job.Status),
	)
	return nil
}

// ─── Subscription Activation (Remnawave) ─────────────────────────────────────

func (w *Worker) handleSubscriptionActivate(ctx context.Context, payload string) error {
	var job SubscriptionActivateJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return err
	}

	userID, _ := uuid.Parse(job.UserID)
	paymentID, _ := uuid.Parse(job.PaymentID)
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

	// Check for existing active subscription to extend
	activeSub, err := w.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		return err
	}

	var newExpiry time.Time
	if activeSub != nil && activeSub.ExpiresAt.After(now) {
		// Extend from current expiry
		newExpiry = activeSub.ExpiresAt.Add(time.Duration(durationDays) * 24 * time.Hour)
	} else {
		newExpiry = now.Add(time.Duration(durationDays) * 24 * time.Hour)
	}

	// Create or update Remnawave user
	var remnaUUID string
	if user.RemnaUserUUID == nil || *user.RemnaUserUUID == "" {
		remnaUser, err := w.remna.CreateUser(ctx, userID.String(), newExpiry)
		if err != nil {
			return fmt.Errorf("create remna user: %w", err)
		}
		remnaUUID = remnaUser.UUID
		if err := w.repo.UpdateRemnaUUID(ctx, userID, remnaUUID); err != nil {
			w.log.Error("update remna uuid", zap.Error(err))
		}
	} else {
		remnaUUID = *user.RemnaUserUUID
		if err := w.remna.UpdateExpiry(ctx, remnaUUID, newExpiry); err != nil {
			return fmt.Errorf("update remna expiry: %w", err)
		}
		// Re-enable user in case they were disabled
		if err := w.remna.EnableUser(ctx, remnaUUID); err != nil {
			w.log.Warn("enable remna user failed", zap.Error(err))
		}
	}

	pid := paymentID
	if activeSub != nil {
		// Extend in place
		if err := w.repo.ExtendSubscription(ctx, nil, activeSub.ID, newExpiry); err != nil {
			return err
		}
	} else {
		// Create new subscription record
		sub := &domain.Subscription{
			ID:           uuid.New(),
			UserID:       userID,
			Plan:         plan,
			Status:       domain.SubStatusActive,
			StartsAt:     now,
			ExpiresAt:    newExpiry,
			RemnaSubUUID: &remnaUUID,
			PaidKopecks:  job.AmountKopecks,
			PaymentID:    &pid,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := w.repo.CreateSubscription(ctx, nil, sub); err != nil {
			return err
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

// ─── Referral Reward ──────────────────────────────────────────────────────────
// LTV = total kopecks paid by the referee
// Reward = 15% of payment amount in YAD  (1 YAD = 2.5 ₽)
// 100% paid immediately (after 24–72h delay)

func (w *Worker) handleReferralReward(ctx context.Context, payload string) error {
	var job ReferralRewardJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return err
	}

	refereeID, _ := uuid.Parse(job.RefereeID)
	paymentID, _ := uuid.Parse(job.PaymentID)

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

	// Calculate reward: 15% of payment in rubles → convert to YAD
	// 1 YAD = 2.5 ₽ = 250 kopecks
	const yadPerKopeck = 1.0 / 250.0
	const referralPct = 0.15
	rewardKopecks := int64(float64(job.PaidKopecks) * referralPct)
	totalYAD := int64(float64(rewardKopecks) * yadPerKopeck)
	if totalYAD == 0 {
		totalYAD = 1 // minimum 1 YAD
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

	rewardID, _ := uuid.Parse(job.RewardID)

	// Use Redis lock to prevent double-payout
	lockKey := "reward:payout:" + rewardID.String()
	locked, err := redisrepo.TryLock(ctx, w.rdb, lockKey, 5*time.Minute)
	if err != nil || !locked {
		w.log.Warn("payout lock contention, skipping", zap.String("reward_id", rewardID.String()))
		return nil
	}
	defer redisrepo.Unlock(ctx, w.rdb, lockKey)

	// We operate directly on the DB here via a helper –
	// handled by periodicRewardSweep which queries the rewards table
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
			w.log.Warn("platega status sync failed",
				zap.String("payment_id", p.ID.String()),
				zap.Error(err))
			continue
		}

		newStatus := domain.PaymentStatus(platResp.Status)
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

// periodicExpirySweep marks expired subscriptions and disables VPN access
func (w *Worker) periodicExpirySweep(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := w.repo.UpdateExpiredSubscriptions(ctx)
			if err != nil {
				w.log.Error("expiry sweep failed", zap.Error(err))
				continue
			}
			if n > 0 {
				w.log.Info("marked subscriptions expired", zap.Int64("count", n))
				// Disable Remnawave access for each expired user
				userIDs, err := w.repo.GetExpiredSubscriptionUserIDs(ctx)
				if err != nil {
					w.log.Error("get expired user ids", zap.Error(err))
					continue
				}
				for _, uid := range userIDs {
					user, err := w.repo.GetByID(ctx, uid)
					if err != nil || user == nil || user.RemnaUserUUID == nil {
						continue
					}
					if err := w.remna.DisableUser(ctx, *user.RemnaUserUUID); err != nil {
						w.log.Error("disable remnawave user", zap.String("uid", uid.String()), zap.Error(err))
					}
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
		locked, _ := redisrepo.TryLock(ctx, w.rdb, lockKey, 2*time.Minute)
		if !locked {
			continue
		}

		func() {
			defer redisrepo.Unlock(ctx, w.rdb, lockKey)

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
		locked, _ := redisrepo.TryLock(ctx, w.rdb, lockKey, 2*time.Minute)
		if !locked {
			continue
		}

		func() {
			defer redisrepo.Unlock(ctx, w.rdb, lockKey)

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

// handleNotifyTelegram sends a Telegram message via Bot API HTTP call.
func (w *Worker) handleNotifyTelegram(ctx context.Context, payload string) error {
	if w.tgToken == "" {
		return nil
	}
	var job NotifyTelegramJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", w.tgToken)
	body := fmt.Sprintf(`{"chat_id":%d,"text":%q,"parse_mode":"HTML"}`, job.TelegramID, job.Message)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		w.log.Warn("telegram send failed", zap.Int("status", resp.StatusCode), zap.Int64("tg_id", job.TelegramID))
	}
	return nil
}

// periodicExpiryWarnings sends Telegram notifications 3 days before subscription expires.
func (w *Worker) periodicExpiryWarnings(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
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
		// Deduplicate: only send once per subscription per warning cycle
		dedupKey := fmt.Sprintf("notify:expiry3d:%s", sub.ID.String())
		ok, _ := redisrepo.SetNX(ctx, w.rdb, dedupKey, 7*24*time.Hour)
		if !ok {
			continue
		}
		user, err := w.repo.GetByID(ctx, sub.UserID)
		if err != nil || user == nil || user.TelegramID == nil {
			continue
		}
		daysLeft := int(time.Until(sub.ExpiresAt).Hours() / 24)
		msg := fmt.Sprintf("⚠️ <b>Ваша подписка истекает через %d дн.</b>\n\nПродлите её в личном кабинете, чтобы не потерять доступ к VPN.", daysLeft)
		w.enqueueNotify(ctx, *user.TelegramID, msg)
	}
}
