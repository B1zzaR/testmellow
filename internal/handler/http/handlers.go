// Package httphandler implements all HTTP API handlers.
package httphandler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/vpnplatform/internal/domain"
	"github.com/vpnplatform/internal/integration/remnawave"
	"github.com/vpnplatform/internal/middleware"
	"github.com/vpnplatform/internal/repository/postgres"
	"github.com/vpnplatform/internal/service"
	jwtpkg "github.com/vpnplatform/pkg/jwt"
)

// ─── Auth Handler ─────────────────────────────────────────────────────────────

type AuthHandler struct {
	auth   *service.AuthService
	jwtMgr *jwtpkg.Manager
	log    *zap.Logger
}

func NewAuthHandler(auth *service.AuthService, jwtMgr *jwtpkg.Manager, log *zap.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, jwtMgr: jwtMgr, log: log}
}

type registerRequest struct {
	Username          string `json:"username" binding:"required,min=3,max=64"`
	Password          string `json:"password" binding:"required,min=8"`
	ReferralCode      string `json:"referral_code"`
	DeviceFingerprint string `json:"device_fingerprint"`
}

// POST /api/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.auth.Register(c.Request.Context(), service.RegisterInput{
		Username:          req.Username,
		Password:          req.Password,
		ReferralCode:      req.ReferralCode,
		DeviceFingerprint: req.DeviceFingerprint,
		IP:                c.ClientIP(),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.jwtMgr.Generate(user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":         token,
		"user_id":       user.ID,
		"referral_code": user.ReferralCode,
	})
}

type loginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required"`
}

// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.auth.Login(c.Request.Context(), service.LoginInput{
		Username: req.Username,
		Password: req.Password,
		IP:       c.ClientIP(),
	})
	if err != nil {
		// Generic message to prevent user enumeration
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.jwtMgr.Generate(user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"user_id": user.ID,
	})
}

// ─── Profile Handler ──────────────────────────────────────────────────────────

type ProfileHandler struct {
	repo  *postgres.UserRepo
	remna *remnawave.Client
	log   *zap.Logger
}

func NewProfileHandler(repo *postgres.UserRepo, remna *remnawave.Client, log *zap.Logger) *ProfileHandler {
	return &ProfileHandler{repo: repo, remna: remna, log: log}
}

// GET /api/profile
func (h *ProfileHandler) Get(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	user, err := h.repo.GetByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            user.ID,
		"email":         user.Email,
		"username":      user.Username,
		"yad_balance":   user.YADBalance,
		"referral_code": user.ReferralCode,
		"ltv_kopecks":   user.LTV,
		"trial_used":    user.TrialUsed,
		"is_admin":      user.IsAdmin,
		"is_banned":     user.IsBanned,
		"risk_score":    user.RiskScore,
		"created_at":    user.CreatedAt,
	})
}

// GET /api/profile/connection
// Returns the Remnawave subscribe URL for the authenticated user.
func (h *ProfileHandler) GetConnection(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.CurrentUserID(c)
	user, err := h.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	remnaUUID := ""
	if user.RemnaUserUUID != nil {
		remnaUUID = *user.RemnaUserUUID
	}

	// Lazy repair: if remna_user_uuid is missing, try to recover it from the
	// subscription's remna_sub_uuid (which is the same Remnawave user UUID).
	if remnaUUID == "" {
		subs, subErr := h.repo.GetActiveSubscription(ctx, userID)
		if subErr != nil || subs == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "no active subscription"})
			return
		}

		// Path 1: subscription has a stored remna_sub_uuid — use it directly
		if subs.RemnaSubUUID != nil && *subs.RemnaSubUUID != "" {
			remnaUUID = *subs.RemnaSubUUID
			_ = h.repo.UpdateRemnaUUID(ctx, userID, remnaUUID)
		} else {
			// Path 2: look up by username in Remnawave
			remnaUser, lookupErr := h.remna.GetUserByUsername(ctx, userID.String())
			if lookupErr == nil && remnaUser != nil && remnaUser.UUID != "" {
				remnaUUID = remnaUser.UUID
				_ = h.repo.UpdateRemnaUUID(ctx, userID, remnaUUID)
				c.JSON(http.StatusOK, gin.H{"subscribe_url": remnaUser.SubscribeURL})
				return
			}
			// Path 3: create a new Remnawave account
			remnaUser, createErr := h.remna.CreateUser(ctx, userID.String(), subs.ExpiresAt)
			if createErr != nil || remnaUser == nil || remnaUser.UUID == "" {
				h.log.Warn("remnawave lazy repair: create user failed", zap.Error(createErr))
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "could not provision vpn account"})
				return
			}
			remnaUUID = remnaUser.UUID
			_ = h.repo.UpdateRemnaUUID(ctx, userID, remnaUUID)
			c.JSON(http.StatusOK, gin.H{"subscribe_url": remnaUser.SubscribeURL})
			return
		}
	}

	remnaUser, err := h.remna.GetUser(ctx, remnaUUID)
	if err != nil {
		h.log.Warn("remnawave get user", zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "could not fetch connection info"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subscribe_url": remnaUser.SubscribeURL})
}

// ─── Balance Handler ──────────────────────────────────────────────────────────

type BalanceHandler struct {
	repo *postgres.UserRepo
	log  *zap.Logger
}

func NewBalanceHandler(repo *postgres.UserRepo, log *zap.Logger) *BalanceHandler {
	return &BalanceHandler{repo: repo, log: log}
}

// GET /api/balance
func (h *BalanceHandler) Get(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	user, err := h.repo.GetByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 1 YAD = 2.5 ₽ = 250 kopecks
	const yadToKopecks = 250
	c.JSON(http.StatusOK, gin.H{
		"yad_balance":     user.YADBalance,
		"yad_ruble_value": float64(user.YADBalance) * 2.5,
		"yad_to_kopecks":  yadToKopecks,
	})
}

// GET /api/balance/history
func (h *BalanceHandler) History(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	txs, err := h.repo.GetYADTransactions(c.Request.Context(), userID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load history"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"transactions": txs})
}

// ─── Subscription Handler ─────────────────────────────────────────────────────

type SubscriptionHandler struct {
	svc *service.SubscriptionService
	log *zap.Logger
}

func NewSubscriptionHandler(svc *service.SubscriptionService, log *zap.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc, log: log}
}

// GET /api/subscriptions
func (h *SubscriptionHandler) List(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	subs, err := h.svc.GetUserSubscriptions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscriptions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subscriptions": subs})
}

// GET /api/subscriptions/:id
func (h *SubscriptionHandler) GetByID(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	subID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription id"})
		return
	}

	subs, err := h.svc.GetUserSubscriptions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscriptions"})
		return
	}
	for _, s := range subs {
		if s.ID == subID {
			c.JSON(http.StatusOK, s)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
}

type buySubscriptionRequest struct {
	Plan      string `json:"plan" binding:"required"`
	ReturnURL string `json:"return_url"`
}

// POST /api/subscriptions/buy
func (h *SubscriptionHandler) Buy(c *gin.Context) {
	var req buySubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.CurrentUserID(c)
	plan := domain.SubscriptionPlan(req.Plan)

	redirect, payment, err := h.svc.InitiatePayment(c.Request.Context(), userID, plan, req.ReturnURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"payment_id":   payment.ID,
		"redirect_url": redirect,
		"amount_rub":   float64(payment.AmountKopecks) / 100,
		"plan":         req.Plan,
		"expires_in":   "15 minutes",
	})
}

// POST /api/subscriptions/renew
func (h *SubscriptionHandler) Renew(c *gin.Context) {
	var req buySubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.CurrentUserID(c)
	plan := domain.SubscriptionPlan(req.Plan)

	redirect, payment, err := h.svc.InitiateRenewal(c.Request.Context(), userID, plan, req.ReturnURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"payment_id":   payment.ID,
		"redirect_url": redirect,
		"amount_rub":   float64(payment.AmountKopecks) / 100,
		"plan":         req.Plan,
	})
}

// ─── Payment Handler ─────────────────────────────────────────────────────────

type PaymentHandler struct {
	svc *service.SubscriptionService
	log *zap.Logger
}

func NewPaymentHandler(svc *service.SubscriptionService, log *zap.Logger) *PaymentHandler {
	return &PaymentHandler{svc: svc, log: log}
}

// GET /api/payments/pending
func (h *PaymentHandler) ListPending(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	payments, err := h.svc.GetPendingPayments(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load payments"})
		return
	}
	if payments == nil {
		payments = []*domain.Payment{}
	}
	c.JSON(http.StatusOK, gin.H{"payments": payments})
}

// GET /api/payments/:id
func (h *PaymentHandler) GetByID(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment id"})
		return
	}
	payment, err := h.svc.GetPaymentByID(c.Request.Context(), userID, paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, payment)
}

// POST /api/payments/:id/check — manually syncs status with Platega
func (h *PaymentHandler) Check(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment id"})
		return
	}
	payment, err := h.svc.CheckPaymentStatus(c.Request.Context(), userID, paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, payment)
}

// ─── Referral Handler ────────────────────────────────────────────────────────

type ReferralHandler struct {
	repo *postgres.UserRepo
	log  *zap.Logger
}

func NewReferralHandler(repo *postgres.UserRepo, log *zap.Logger) *ReferralHandler {
	return &ReferralHandler{repo: repo, log: log}
}

// GET /api/referrals
func (h *ReferralHandler) List(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	refs, err := h.repo.GetReferralsByReferrer(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load referrals"})
		return
	}

	user, _ := h.repo.GetByID(c.Request.Context(), userID)
	referralCode := ""
	if user != nil {
		referralCode = user.ReferralCode
	}

	c.JSON(http.StatusOK, gin.H{
		"referral_code":  referralCode,
		"referral_count": len(refs),
		"referrals":      refs,
	})
}

// ─── Promo Handler ────────────────────────────────────────────────────────────

type PromoHandler struct {
	eco *service.EconomyService
	log *zap.Logger
}

func NewPromoHandler(eco *service.EconomyService, log *zap.Logger) *PromoHandler {
	return &PromoHandler{eco: eco, log: log}
}

type usePromoRequest struct {
	Code string `json:"code" binding:"required"`
}

// POST /api/promo/use
func (h *PromoHandler) Use(c *gin.Context) {
	var req usePromoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.CurrentUserID(c)
	promo, err := h.eco.UsePromoCode(c.Request.Context(), userID, req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Promo code applied",
		"yad_earned": promo.YADAmount,
	})
}

// ─── Trial Handler ────────────────────────────────────────────────────────────

type TrialHandler struct {
	trial *service.TrialService
	log   *zap.Logger
}

func NewTrialHandler(trial *service.TrialService, log *zap.Logger) *TrialHandler {
	return &TrialHandler{trial: trial, log: log}
}

// POST /api/trial/activate
func (h *TrialHandler) Activate(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	sub, err := h.trial.ActivateTrial(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":    "Trial activated",
		"expires_at": sub.ExpiresAt,
		"status":     sub.Status,
	})
}

// ─── Ticket Handler ───────────────────────────────────────────────────────────

type TicketHandler struct {
	repo *postgres.UserRepo
	log  *zap.Logger
}

func NewTicketHandler(repo *postgres.UserRepo, log *zap.Logger) *TicketHandler {
	return &TicketHandler{repo: repo, log: log}
}

type createTicketRequest struct {
	Subject string `json:"subject" binding:"required,min=3,max=256"`
	Message string `json:"message" binding:"required,min=1"`
}

// POST /api/tickets
func (h *TicketHandler) Create(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	var req createTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	ticket := &domain.Ticket{
		ID:        uuid.New(),
		UserID:    userID,
		Subject:   req.Subject,
		Status:    domain.TicketOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.repo.CreateTicket(c.Request.Context(), ticket); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create ticket"})
		return
	}

	msg := &domain.TicketMessage{
		ID:        uuid.New(),
		TicketID:  ticket.ID,
		SenderID:  userID,
		IsAdmin:   false,
		Body:      req.Message,
		CreatedAt: now,
	}
	if err := h.repo.AddTicketMessage(c.Request.Context(), msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add message"})
		return
	}

	c.JSON(http.StatusCreated, ticket)
}

// GET /api/tickets
func (h *TicketHandler) List(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	tickets, err := h.repo.ListTickets(c.Request.Context(), &userID, "", 20, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tickets"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tickets": tickets})
}

// GET /api/tickets/:id
func (h *TicketHandler) GetWithMessages(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}

	ticket, err := h.repo.GetTicketByID(c.Request.Context(), ticketID)
	if err != nil || ticket == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return
	}
	// Access control: user can only see their own tickets
	if ticket.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	msgs, err := h.repo.GetTicketMessages(c.Request.Context(), ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ticket": ticket, "messages": msgs})
}

type replyTicketRequest struct {
	Message string `json:"message" binding:"required,min=1"`
}

// POST /api/tickets/:id/reply
func (h *TicketHandler) Reply(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}

	var req replyTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ticket, err := h.repo.GetTicketByID(c.Request.Context(), ticketID)
	if err != nil || ticket == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return
	}
	if ticket.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if ticket.Status == domain.TicketClosed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticket is closed"})
		return
	}

	msg := &domain.TicketMessage{
		ID:        uuid.New(),
		TicketID:  ticketID,
		SenderID:  userID,
		IsAdmin:   false,
		Body:      req.Message,
		CreatedAt: time.Now(),
	}
	if err := h.repo.AddTicketMessage(c.Request.Context(), msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "reply added"})
}

// ─── Shop Handler ─────────────────────────────────────────────────────────────

type ShopHandler struct {
	repo *postgres.UserRepo
	eco  *service.EconomyService
	log  *zap.Logger
}

func NewShopHandler(repo *postgres.UserRepo, eco *service.EconomyService, log *zap.Logger) *ShopHandler {
	return &ShopHandler{repo: repo, eco: eco, log: log}
}

// GET /api/shop
func (h *ShopHandler) List(c *gin.Context) {
	items, err := h.repo.ListShopItems(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load shop"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type buyItemRequest struct {
	ItemID   string `json:"item_id" binding:"required"`
	Quantity int    `json:"quantity" binding:"required,min=1"`
}

// POST /api/shop/buy
func (h *ShopHandler) Buy(c *gin.Context) {
	var req buyItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	itemID, err := uuid.Parse(req.ItemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	userID := middleware.CurrentUserID(c)
	order, err := h.eco.BuyShopItem(c.Request.Context(), userID, itemID, req.Quantity)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_id":  order.ID,
		"total_yad": order.TotalYAD,
		"message":   "Purchase successful",
	})
}
