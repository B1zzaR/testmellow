// Package webhook implements the Platega payment callback endpoint.
//
// Platega documentation:
//   POST your-callback-url
//   Headers: X-MerchantId, X-Secret
//   Body: { id, amount, currency, status, paymentMethod, payload }
//   Statuses: CONFIRMED | CANCELED | CHARGEBACKED
//   Retry: up to 3 times with 5-minute intervals if no 200 response within 60s.
//
// Security:
//   1. Verify X-MerchantId and X-Secret match our stored credentials.
//   2. Use DB-level idempotency (webhook_events table, UNIQUE source+external_id+event_type).
//   3. All financial processing delegated to the worker queue — never inline.
package webhook

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/vpnplatform/internal/integration/platega"
	"github.com/vpnplatform/internal/repository/postgres"
	redisrepo "github.com/vpnplatform/internal/repository/redis"
	"github.com/vpnplatform/internal/worker"
)

type PlategalHandler struct {
	platega *platega.Client
	repo    *postgres.UserRepo
	rdb     *redis.Client
	log     *zap.Logger
}

func NewPlategalHandler(
	pc *platega.Client,
	repo *postgres.UserRepo,
	rdb *redis.Client,
	log *zap.Logger,
) *PlategalHandler {
	return &PlategalHandler{platega: pc, repo: repo, rdb: rdb, log: log}
}

// Handle processes POST /webhooks/platega
//
// This is the callback endpoint registered in the Platega dashboard.
// All payment state changes arrive here.
func (h *PlategalHandler) Handle(c *gin.Context) {
	// ── Step 1: Authenticate the webhook ─────────────────────────────────
	merchantID := c.GetHeader("X-MerchantId")
	secret := c.GetHeader("X-Secret")

	if !h.platega.VerifyWebhookHeaders(merchantID, secret) {
		h.log.Warn("platega webhook auth failed",
			zap.String("merchant_id", merchantID),
			zap.String("ip", c.ClientIP()),
		)
		// Return 200 to prevent Platega from treating this as a transport failure.
		// We silently discard unauthenticated requests.
		c.Status(http.StatusOK)
		return
	}

	// ── Step 2: Parse body ────────────────────────────────────────────────
	var cb platega.CallbackPayload
	if err := c.ShouldBindJSON(&cb); err != nil {
		h.log.Warn("platega webhook invalid body", zap.Error(err))
		c.Status(http.StatusOK) // return 200 to avoid retries on malformed data
		return
	}

	rawBody, _ := json.Marshal(cb)

	h.log.Info("platega webhook received",
		zap.String("transaction_id", cb.ID),
		zap.String("status", string(cb.Status)),
	)

	// ── Step 3: Idempotency (DB level) ────────────────────────────────────
	isNew, err := h.repo.EnsureWebhookEvent(
		c.Request.Context(), "platega", cb.ID, string(cb.Status), rawBody,
	)
	if err != nil {
		h.log.Error("webhook idempotency check failed", zap.Error(err))
		c.Status(http.StatusOK)
		return
	}
	if !isNew {
		h.log.Info("duplicate platega webhook, skipping",
			zap.String("transaction_id", cb.ID),
			zap.String("status", string(cb.Status)),
		)
		c.Status(http.StatusOK)
		return
	}

	// ── Step 4: Look up the payment to get user and plan ──────────────────
	// The payment was created when the user initiated checkout.
	// Payment ID = Platega transaction ID.
	paymentID, err := uuid.Parse(cb.ID)
	if err != nil {
		h.log.Error("invalid transaction uuid from Platega", zap.String("id", cb.ID))
		_ = h.repo.MarkWebhookProcessed(c.Request.Context(), "platega", cb.ID, string(cb.Status), "invalid uuid")
		c.Status(http.StatusOK)
		return
	}

	payment, err := h.repo.GetPaymentByID(c.Request.Context(), paymentID)
	if err != nil || payment == nil {
		h.log.Error("payment not found for webhook",
			zap.String("transaction_id", cb.ID),
		)
		// Record error, still return 200 so Platega doesn't retry indefinitely
		_ = h.repo.MarkWebhookProcessed(c.Request.Context(), "platega", cb.ID, string(cb.Status), "payment not found")
		c.Status(http.StatusOK)
		return
	}

	// Only process CONFIRMED and CANCELED — ignore duplicates at job level too
	if cb.Status != platega.StatusConfirmed && cb.Status != platega.StatusCanceled && cb.Status != platega.StatusChargebacked {
		_ = h.repo.MarkWebhookProcessed(c.Request.Context(), "platega", cb.ID, string(cb.Status), "")
		c.Status(http.StatusOK)
		return
	}

	// ── Step 5: Enqueue payment processing to worker ──────────────────────
	// Amount from Platega is in rubles; convert to kopecks for internal use.
	amountKopecks := int64(cb.Amount * 100)

	job := worker.PaymentProcessJob{
		TransactionID: cb.ID,
		UserID:        payment.UserID.String(),
		AmountKopecks: amountKopecks,
		Plan:          string(payment.Plan),
		Status:        string(cb.Status),
	}

	if err := worker.Enqueue(c.Request.Context(), h.rdb, worker.QueuePaymentProcess, job); err != nil {
		h.log.Error("failed to enqueue payment job", zap.Error(err))
		// Do NOT return error — we'll re-process via status check cron if needed.
		// Mark as unprocessed so sweep can retry.
		_ = h.repo.MarkWebhookProcessed(c.Request.Context(), "platega", cb.ID, string(cb.Status), err.Error())
		c.Status(http.StatusOK)
		return
	}

	// ── Step 6: Mark webhook as processed ────────────────────────────────
	_ = h.repo.MarkWebhookProcessed(c.Request.Context(), "platega", cb.ID, string(cb.Status), "")

	// Rate-limit protection: record the queue addition in Redis so we
	// don't accidentally enqueue the same transaction twice from different paths.
	_, _ = redisrepo.MarkPaymentQueued(c.Request.Context(), h.rdb, cb.ID)

	h.log.Info("platega webhook enqueued",
		zap.String("transaction_id", cb.ID),
		zap.String("status", string(cb.Status)),
		zap.String("user_id", payment.UserID.String()),
	)

	// Must return 200 OK to acknowledge receipt — Platega stops retrying on 200.
	c.Status(http.StatusOK)
}
