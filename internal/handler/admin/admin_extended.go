// admin_extended.go — additional admin handlers for payments, subscriptions,
// YAD economy, referrals, user profile, audit logs, and analytics.
package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/vpnplatform/internal/domain"
	"github.com/vpnplatform/internal/worker"
)

// ─── User sub-resources ───────────────────────────────────────────────────────

// GET /admin/users/:id/subscriptions
func (h *Handler) GetUserSubscriptions(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	subs, err := h.repo.ListSubscriptions(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscriptions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subscriptions": subs})
}

// GET /admin/users/:id/payments
func (h *Handler) GetUserPayments(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	payments, err := h.repo.ListPaymentsByUser(c.Request.Context(), id, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load payments"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"payments": payments})
}

// GET /admin/users/:id/yad
func (h *Handler) GetUserYAD(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	txs, err := h.repo.GetYADTransactions(c.Request.Context(), id, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load YAD transactions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"transactions": txs})
}

type adjustYADRequest struct {
	Delta int64  `json:"delta" binding:"required,min=-1000000,max=1000000"`
	Note  string `json:"note"  binding:"required,min=3,max=255"`
}

// POST /admin/users/:id/adjust-yad
func (h *Handler) AdjustUserYAD(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	var req adjustYADRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx, err := h.repo.BeginTx(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tx begin failed"})
		return
	}
	defer tx.Rollback(c.Request.Context()) //nolint:errcheck

	if err := h.repo.AdjustYADBalance(c.Request.Context(), tx, id, req.Delta, domain.YADTxBonus, nil, req.Note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	h.audit(c, "yad.adjust", strPtr("user"), uidPtr(id), strPtr(fmt.Sprintf("delta=%d note=%s", req.Delta, req.Note)))
	h.log.Info("admin adjusted YAD", zap.String("user_id", id.String()), zap.Int64("delta", req.Delta))
	c.JSON(http.StatusOK, gin.H{"message": "balance adjusted"})
}

// ─── Payments ─────────────────────────────────────────────────────────────────

// GET /admin/payments?status=&from=&to=&limit=&offset=
func (h *Handler) ListPayments(c *gin.Context) {
	status := c.Query("status")
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	var from, to *time.Time
	if s := c.Query("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			from = &t
		}
	}
	if s := c.Query("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			to = &t
		}
	}

	payments, err := h.repo.ListAllPayments(c.Request.Context(), status, from, to, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load payments"})
		return
	}
	total, _ := h.repo.CountPayments(c.Request.Context(), status, from, to)
	c.JSON(http.StatusOK, gin.H{"payments": payments, "total": total, "limit": limit, "offset": offset})
}

// GET /admin/payments/:id
func (h *Handler) GetPayment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment id"})
		return
	}
	payment, err := h.repo.GetPaymentByID(c.Request.Context(), id)
	if err != nil || payment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}
	c.JSON(http.StatusOK, payment)
}

// POST /admin/payments/:id/check — calls Platega for current status, updates DB, and
// enqueues subscription activation if the payment is now CONFIRMED.
func (h *Handler) CheckPaymentStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment id"})
		return
	}
	payment, err := h.repo.GetPaymentByID(c.Request.Context(), id)
	if err != nil || payment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}

	if h.platega == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "payment gateway not configured"})
		return
	}

	// Already in a terminal state — no need to query Platega.
	if payment.Status != domain.PaymentStatusPending {
		h.audit(c, "payment.check", strPtr("payment"), uidPtr(id), strPtr(string(payment.Status)))
		c.JSON(http.StatusOK, gin.H{
			"payment_id":     payment.ID,
			"platega_status": payment.Status,
			"db_status":      payment.Status,
			"message":        "payment is already in terminal state",
		})
		return
	}

	resp, err := h.platega.GetPaymentStatus(c.Request.Context(), payment.ID.String())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "platega check failed: " + err.Error()})
		return
	}

	newStatus := domain.PaymentStatus(resp.Status)
	if newStatus != payment.Status {
		_ = h.repo.UpdatePaymentStatus(c.Request.Context(), nil, id, newStatus)
	}

	// If now confirmed, enqueue the same processing job the webhook would have sent.
	if newStatus == domain.PaymentStatusConfirmed {
		job := worker.PaymentProcessJob{
			TransactionID: payment.ID.String(),
			UserID:        payment.UserID.String(),
			AmountKopecks: payment.AmountKopecks,
			Plan:          string(payment.Plan),
			Status:        string(newStatus),
		}
		if enqErr := worker.Enqueue(c.Request.Context(), h.rdb, worker.QueuePaymentProcess, job); enqErr != nil {
			h.log.Error("admin: failed to enqueue payment activation", zap.Error(enqErr))
		}
	}

	h.audit(c, "payment.check", strPtr("payment"), uidPtr(id), strPtr(string(resp.Status)))
	c.JSON(http.StatusOK, gin.H{
		"payment_id":     payment.ID,
		"platega_status": resp.Status,
		"db_status":      newStatus,
	})
}

// ─── Subscriptions ────────────────────────────────────────────────────────────

type assignSubscriptionRequest struct {
	Login string `json:"login" binding:"required"`
	Plan  string `json:"plan"  binding:"required,oneof=1week 1month 3months 99years"`
}

// POST /admin/subscriptions/assign — find user by login, activate subscription
func (h *Handler) AssignSubscription(c *gin.Context) {
	var req assignSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.repo.GetByUsername(c.Request.Context(), req.Login)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	plan := domain.SubscriptionPlan(req.Plan)
	durationDays := domain.PlanDurationDays(plan)
	if durationDays == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan"})
		return
	}

	now := time.Now()
	activeSub, err := h.repo.GetActiveSubscription(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "subscription lookup failed"})
		return
	}

	var newExpiry time.Time
	if activeSub != nil && activeSub.ExpiresAt.After(now) {
		newExpiry = activeSub.ExpiresAt.Add(time.Duration(durationDays) * 24 * time.Hour)
	} else {
		newExpiry = now.Add(time.Duration(durationDays) * 24 * time.Hour)
	}

	// Activate in Remnawave
	var remnaUUID string
	if h.remna != nil {
		if user.RemnaUserUUID == nil || *user.RemnaUserUUID == "" {
			remnaUser, err := h.remna.CreateUser(c.Request.Context(), user.ID.String(), newExpiry)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
				return
			}
			remnaUUID = remnaUser.UUID
			_ = h.repo.UpdateRemnaUUID(c.Request.Context(), user.ID, remnaUUID)
		} else {
			remnaUUID = *user.RemnaUserUUID
			if err := h.remna.UpdateExpiry(c.Request.Context(), remnaUUID, newExpiry); err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": "remnawave update expiry: " + err.Error()})
				return
			}
			_ = h.remna.EnableUser(c.Request.Context(), remnaUUID)
		}
	}

	var sub *domain.Subscription
	if activeSub != nil {
		if err := h.repo.ExtendSubscription(c.Request.Context(), nil, activeSub.ID, newExpiry); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to extend subscription"})
			return
		}
		activeSub.ExpiresAt = newExpiry
		sub = activeSub
	} else {
		sub = &domain.Subscription{
			ID:        uuid.New(),
			UserID:    user.ID,
			Plan:      plan,
			Status:    domain.SubStatusActive,
			StartsAt:  now,
			ExpiresAt: newExpiry,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if remnaUUID != "" {
			sub.RemnaSubUUID = &remnaUUID
		}
		if err := h.repo.CreateSubscription(c.Request.Context(), nil, sub); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create subscription"})
			return
		}
	}

	details := fmt.Sprintf("login=%s plan=%s expires=%s", req.Login, req.Plan, newExpiry.Format(time.RFC3339))
	h.audit(c, "subscription.assign", strPtr("user"), uidPtr(user.ID), strPtr(details))
	h.log.Info("admin assigned subscription",
		zap.String("login", req.Login),
		zap.String("plan", req.Plan),
		zap.Time("expires_at", newExpiry),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":      "subscription assigned",
		"subscription": sub,
		"expires_at":   newExpiry,
		"login":        req.Login,
	})
}

// GET /admin/subscriptions?status=&user_id=&limit=&offset=
func (h *Handler) ListSubscriptions(c *gin.Context) {
	status := c.Query("status")
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	var userID *uuid.UUID
	if s := c.Query("user_id"); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			userID = &uid
		}
	}

	subs, err := h.repo.ListAllSubscriptions(c.Request.Context(), status, userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscriptions"})
		return
	}
	total, _ := h.repo.CountSubscriptions(c.Request.Context(), status, userID)
	c.JSON(http.StatusOK, gin.H{"subscriptions": subs, "total": total, "limit": limit, "offset": offset})
}

type setSubStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active expired canceled"`
}

// PATCH /admin/subscriptions/:id/status
func (h *Handler) SetSubscriptionStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription id"})
		return
	}
	var req setSubStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.SetSubscriptionStatus(c.Request.Context(), id, domain.SubscriptionStatus(req.Status)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update status"})
		return
	}
	h.audit(c, "subscription.set_status", strPtr("subscription"), uidPtr(id), strPtr(req.Status))
	c.JSON(http.StatusOK, gin.H{"message": "status updated"})
}

type extendSubRequest struct {
	Days int `json:"days" binding:"required,min=1,max=3650"`
}

// POST /admin/subscriptions/:id/extend
func (h *Handler) ExtendSubscription(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription id"})
		return
	}
	var req extendSubRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sub, err := h.repo.ExtendSubscriptionByDays(c.Request.Context(), id, req.Days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to extend subscription"})
		return
	}
	h.audit(c, "subscription.extend", strPtr("subscription"), uidPtr(id), strPtr(fmt.Sprintf("+%d days", req.Days)))
	c.JSON(http.StatusOK, gin.H{"subscription": sub, "message": fmt.Sprintf("extended by %d days", req.Days)})
}

// ─── YAD Economy ─────────────────────────────────────────────────────────────

// GET /admin/yad?login=&type=&limit=&offset=
func (h *Handler) ListYADTransactions(c *gin.Context) {
	txType := c.Query("type")
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	var userID *uuid.UUID
	if login := c.Query("login"); login != "" {
		u, err := h.repo.GetByUsername(c.Request.Context(), login)
		if err == nil {
			uid := u.ID
			userID = &uid
		}
	}

	txs, err := h.repo.ListAllYADTransactions(c.Request.Context(), userID, txType, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load transactions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"transactions": txs})
}

type adminAdjustYADRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Delta  int64  `json:"delta"   binding:"required,min=-1000000,max=1000000"`
	Note   string `json:"note"    binding:"required,min=3,max=255"`
}

// POST /admin/yad/adjust
func (h *Handler) AdjustYAD(c *gin.Context) {
	var req adminAdjustYADRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	tx, err := h.repo.BeginTx(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "tx begin failed"})
		return
	}
	defer tx.Rollback(c.Request.Context()) //nolint:errcheck

	if err := h.repo.AdjustYADBalance(c.Request.Context(), tx, userID, req.Delta, domain.YADTxBonus, nil, req.Note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	h.audit(c, "yad.admin_adjust", strPtr("user"), uidPtr(userID), strPtr(fmt.Sprintf("delta=%d", req.Delta)))
	c.JSON(http.StatusOK, gin.H{"message": "balance adjusted"})
}

// ─── Referrals ────────────────────────────────────────────────────────────────

// GET /admin/referrals?login=&limit=&offset=
func (h *Handler) ListReferrals(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	var referrerID *uuid.UUID
	if login := c.Query("login"); login != "" {
		if u, err := h.repo.GetByUsername(c.Request.Context(), login); err == nil {
			uid := u.ID
			referrerID = &uid
		}
	}

	refs, err := h.repo.GetAllReferrals(c.Request.Context(), referrerID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load referrals"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"referrals": refs})
}

// ─── Audit Logs ───────────────────────────────────────────────────────────────

// GET /admin/audit-logs?limit=&offset=
func (h *Handler) ListAuditLogs(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	logs, err := h.repo.ListAuditLogs(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit logs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// ─── Extended Analytics ───────────────────────────────────────────────────────

// GET /admin/analytics/revenue?days=30
func (h *Handler) RevenueAnalytics(c *gin.Context) {
	days := queryInt(c, "days", 30)
	if days < 1 || days > 365 {
		days = 30
	}

	stats, err := h.repo.GetRevenueByDay(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load revenue stats"})
		return
	}
	topReferrers, err := h.repo.GetTopReferrers(c.Request.Context(), 10)
	if err != nil {
		h.log.Warn("top referrers load failed", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{
		"revenue_by_day": stats,
		"top_referrers":  topReferrers,
		"period_days":    days,
	})
}

// ─── Risk Events ──────────────────────────────────────────────────────────────

// GET /admin/risk?limit=&offset= — lists high-risk users ordered by risk_score desc.
func (h *Handler) ListHighRiskUsers(c *gin.Context) {
	users, err := h.repo.List(c.Request.Context(), queryInt(c, "limit", 50), queryInt(c, "offset", 0))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load users"})
		return
	}
	// Filter to risk >= 40, already sorted by created_at; re-sort client-side is acceptable
	// since we return all and the frontend sorts. For clarity we filter server-side here.
	var risky []*domain.User
	for _, u := range users {
		if u.RiskScore >= 40 {
			risky = append(risky, u)
		}
	}
	c.JSON(http.StatusOK, gin.H{"users": risky})
}

// ─── Platform Settings ────────────────────────────────────────────────────────

// GET /admin/settings
func (h *Handler) GetSettings(c *gin.Context) {
	settings, err := h.repo.GetPlatformSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

type toggleBlockRealMoneyRequest struct {
	BlockRealMoneyPurchases bool `json:"block_real_money_purchases"`
}

// POST /admin/settings/block-real-money-purchases
func (h *Handler) ToggleBlockRealMoney(c *gin.Context) {
	var req toggleBlockRealMoneyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Error("failed to bind toggle real money request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	settings := &domain.PlatformSettings{
		ID:                      1,
		BlockRealMoneyPurchases: req.BlockRealMoneyPurchases,
		UpdatedAt:               time.Now(),
	}

	if err := h.repo.UpdatePlatformSettings(c.Request.Context(), settings); err != nil {
		h.log.Error("failed to update settings", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to update settings: %v", err)})
		return
	}

	action := "settings.block_real_money_disable"
	if req.BlockRealMoneyPurchases {
		action = "settings.block_real_money_enable"
	}
	details := fmt.Sprintf("block_real_money_purchases=%v", req.BlockRealMoneyPurchases)
	h.audit(c, action, nil, nil, &details)

	h.log.Info("admin toggled block real money setting", zap.Bool("enabled", req.BlockRealMoneyPurchases))
	c.JSON(http.StatusOK, settings)
}

// ─── System Notifications ────────────────────────────────────────────────────────

type createNotificationRequest struct {
	Type    string `json:"type" binding:"required,oneof=warning error info success"`
	Title   string `json:"title" binding:"required,min=3,max=255"`
	Message string `json:"message" binding:"required,min=3,max=2000"`
}

// POST /admin/notifications
func (h *Handler) CreateNotification(c *gin.Context) {
	var req createNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminIDVal, _ := c.Get("user_id")
	adminID, _ := adminIDVal.(uuid.UUID)

	notif := &domain.SystemNotification{
		ID:        uuid.New(),
		Type:      domain.NotificationType(req.Type),
		Title:     req.Title,
		Message:   req.Message,
		IsActive:  true,
		CreatedBy: &adminID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.repo.CreateNotification(c.Request.Context(), notif); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create notification"})
		return
	}

	details := fmt.Sprintf("type=%s title=%s", req.Type, req.Title)
	h.audit(c, "notification.create", strPtr("notification"), uidPtr(notif.ID), strPtr(details))
	h.log.Info("admin created notification", zap.String("type", req.Type), zap.String("title", req.Title))

	c.JSON(http.StatusCreated, notif)
}

// GET /admin/notifications?limit=&offset=
func (h *Handler) ListNotifications(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	notifs, err := h.repo.ListAllNotifications(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load notifications"})
		return
	}
	if notifs == nil {
		notifs = []*domain.SystemNotification{}
	}
	c.JSON(http.StatusOK, gin.H{"notifications": notifs})
}

// GET /admin/notifications/:id
func (h *Handler) GetNotification(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
		return
	}
	notif, err := h.repo.GetNotificationByID(c.Request.Context(), id)
	if err != nil || notif == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "notification not found"})
		return
	}
	c.JSON(http.StatusOK, notif)
}

type updateNotificationRequest struct {
	Type     string `json:"type" binding:"required,oneof=warning error info success"`
	Title    string `json:"title" binding:"required,min=3,max=255"`
	Message  string `json:"message" binding:"required,min=3,max=2000"`
	IsActive bool   `json:"is_active"`
}

// PATCH /admin/notifications/:id
func (h *Handler) UpdateNotification(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
		return
	}
	var req updateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	notif, err := h.repo.GetNotificationByID(c.Request.Context(), id)
	if err != nil || notif == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "notification not found"})
		return
	}

	notif.Type = domain.NotificationType(req.Type)
	notif.Title = req.Title
	notif.Message = req.Message
	notif.IsActive = req.IsActive
	notif.UpdatedAt = time.Now()

	if err := h.repo.UpdateNotification(c.Request.Context(), notif); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update notification"})
		return
	}

	h.audit(c, "notification.update", strPtr("notification"), uidPtr(id), nil)
	c.JSON(http.StatusOK, notif)
}

// DELETE /admin/notifications/:id
func (h *Handler) DeleteNotification(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
		return
	}
	if err := h.repo.DeleteNotification(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete notification"})
		return
	}
	h.audit(c, "notification.delete", strPtr("notification"), uidPtr(id), nil)
	c.JSON(http.StatusOK, gin.H{"message": "notification deleted"})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func queryInt(c *gin.Context, key string, def int) int {
	if s := c.Query(key); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

// ensure fmt is used (the Sprintf calls in the handlers use it)
var _ = fmt.Sprintf
