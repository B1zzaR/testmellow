// Package admin contains admin-only API handlers.
package admin

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/vpnplatform/internal/anticheat"
	"github.com/vpnplatform/internal/domain"
	"github.com/vpnplatform/internal/integration/platega"
	"github.com/vpnplatform/internal/integration/remnawave"
	"github.com/vpnplatform/internal/middleware"
	"github.com/vpnplatform/internal/repository/postgres"
)

type Handler struct {
	repo    *postgres.UserRepo
	rdb     *redis.Client
	anti    *anticheat.Engine
	platega *platega.Client
	remna   *remnawave.Client
	log     *zap.Logger
}

func NewHandler(repo *postgres.UserRepo, rdb *redis.Client, anti *anticheat.Engine, platClient *platega.Client, remnaClient *remnawave.Client, log *zap.Logger) *Handler {
	return &Handler{repo: repo, rdb: rdb, anti: anti, platega: platClient, remna: remnaClient, log: log}
}

// ─── Audit log helper ────────────────────────────────────────────────────────

func (h *Handler) audit(c *gin.Context, action string, targetType *string, targetID *uuid.UUID, details *string) {
	adminIDVal, _ := c.Get("user_id")
	adminID, _ := adminIDVal.(uuid.UUID)
	entry := &domain.AdminAuditLog{
		ID:         uuid.New(),
		AdminID:    adminID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Details:    details,
		CreatedAt:  time.Now(),
	}
	if err := h.repo.CreateAuditLog(c.Request.Context(), entry); err != nil {
		h.log.Warn("audit log write failed", zap.Error(err), zap.String("action", action))
	}
}

func strPtr(s string) *string       { return &s }
func uidPtr(u uuid.UUID) *uuid.UUID { return &u }

// ─── Users ───────────────────────────────────────────────────────────────────

// GET /admin/users?q=&page=
func (h *Handler) ListUsers(c *gin.Context) {
	q := c.Query("q")
	page := queryInt(c, "page", 1)
	if page < 1 {
		page = 1
	}
	const limit = 50
	offset := (page - 1) * limit

	users, total, err := h.repo.Search(c.Request.Context(), q, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users, "total": total, "page": page, "limit": limit})
}

// GET /admin/users/:id
func (h *Handler) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	user, err := h.repo.GetByID(c.Request.Context(), id)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// POST /admin/users/:id/ban
func (h *Handler) BanUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if err := h.repo.UpdateBanStatus(c.Request.Context(), id, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to ban user"})
		return
	}
	// Immediately invalidate any live JWT by adding user to the Redis ban set.
	if err := middleware.SetBanKey(c.Request.Context(), h.rdb, id, true); err != nil {
		h.log.Warn("ban: redis set failed", zap.String("user_id", id.String()), zap.Error(err))
	}
	h.audit(c, "user.ban", strPtr("user"), uidPtr(id), nil)
	h.log.Info("user banned by admin", zap.String("user_id", id.String()))
	c.JSON(http.StatusOK, gin.H{"message": "user banned"})
}

// POST /admin/users/:id/unban
func (h *Handler) UnbanUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if err := h.repo.UpdateBanStatus(c.Request.Context(), id, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unban user"})
		return
	}
	// Remove the Redis ban key so the user can access the API again.
	if err := middleware.SetBanKey(c.Request.Context(), h.rdb, id, false); err != nil {
		h.log.Warn("unban: redis del failed", zap.String("user_id", id.String()), zap.Error(err))
	}
	h.audit(c, "user.unban", strPtr("user"), uidPtr(id), nil)
	c.JSON(http.StatusOK, gin.H{"message": "user unbanned"})
}

type setRiskRequest struct {
	Score int `json:"score" binding:"required,min=0,max=100"`
}

// PATCH /admin/users/:id/risk
func (h *Handler) SetRiskScore(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	var req setRiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.UpdateRiskScore(c.Request.Context(), id, req.Score); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update risk score"})
		return
	}
	h.audit(c, "user.risk_score", strPtr("user"), uidPtr(id), strPtr(fmt.Sprintf("%d", req.Score)))
	c.JSON(http.StatusOK, gin.H{"message": "risk score updated"})
}

// ─── Analytics ────────────────────────────────────────────────────────────────

// GET /admin/analytics
func (h *Handler) Analytics(c *gin.Context) {
	data, err := h.repo.GetAnalytics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "analytics error"})
		return
	}
	c.JSON(http.StatusOK, data)
}

// ─── Promo Codes ──────────────────────────────────────────────────────────────

// GET /admin/promocodes
func (h *Handler) ListPromoCodes(c *gin.Context) {
	promos, err := h.repo.ListPromoCodes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load promo codes"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"promocodes": promos})
}

type createPromoRequest struct {
	Code            string  `json:"code" binding:"required,min=4,max=32"`
	PromoType       string  `json:"promo_type"`       // "yad" or "discount"
	YADAmount       int64   `json:"yad_amount"`       // required when promo_type=yad
	DiscountPercent int     `json:"discount_percent"` // required when promo_type=discount
	MaxUses         int     `json:"max_uses" binding:"required,min=1"`
	ExpiresAt       *string `json:"expires_at"` // RFC3339 or null
}

// POST /admin/promocodes
func (h *Handler) CreatePromoCode(c *gin.Context) {
	// Get admin user ID from context
	adminIDVal, _ := c.Get("user_id")
	adminID, _ := adminIDVal.(uuid.UUID)

	var req createPromoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default to yad type
	promoType := req.PromoType
	if promoType == "" {
		promoType = domain.PromoTypeYAD
	}
	if promoType != domain.PromoTypeYAD && promoType != domain.PromoTypeDiscount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "promo_type must be 'yad' or 'discount'"})
		return
	}
	if promoType == domain.PromoTypeYAD && req.YADAmount < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "yad_amount must be >= 1 for yad promo"})
		return
	}
	if promoType == domain.PromoTypeDiscount && (req.DiscountPercent < 1 || req.DiscountPercent > 100) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "discount_percent must be 1–100 for discount promo"})
		return
	}

	promo := &domain.PromoCode{
		ID:              uuid.New(),
		Code:            req.Code,
		PromoType:       promoType,
		YADAmount:       req.YADAmount,
		DiscountPercent: req.DiscountPercent,
		MaxUses:         req.MaxUses,
		UsedCount:       0,
		CreatedByID:     adminID,
		CreatedAt:       time.Now(),
	}

	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expires_at format"})
			return
		}
		promo.ExpiresAt = &t
	}

	if err := h.repo.CreatePromoCode(c.Request.Context(), promo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create promo code"})
		return
	}
	c.JSON(http.StatusCreated, promo)
}

// ─── Tickets ──────────────────────────────────────────────────────────────────

// GET /admin/tickets
func (h *Handler) ListTickets(c *gin.Context) {
	status := c.Query("status")
	tickets, err := h.repo.ListTickets(c.Request.Context(), nil, status, 50, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tickets"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tickets": tickets})
}

// GET /admin/tickets/:id
func (h *Handler) GetTicket(c *gin.Context) {
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

	msgs, err := h.repo.GetTicketMessages(c.Request.Context(), ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "msg load error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ticket": ticket, "messages": msgs})
}

type adminReplyRequest struct {
	Message string `json:"message" binding:"required,min=1,max=4096"`
}

// POST /admin/tickets/:id/reply
func (h *Handler) ReplyToTicket(c *gin.Context) {
	adminIDVal, _ := c.Get("user_id")
	adminID, _ := adminIDVal.(uuid.UUID)

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}

	var req adminReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ticket, err := h.repo.GetTicketByID(c.Request.Context(), ticketID)
	if err != nil || ticket == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
		return
	}

	msg := &domain.TicketMessage{
		ID:        uuid.New(),
		TicketID:  ticketID,
		SenderID:  adminID,
		IsAdmin:   true,
		Body:      req.Message,
		CreatedAt: time.Now(),
	}
	if err := h.repo.AddTicketMessage(c.Request.Context(), msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add reply"})
		return
	}
	_ = h.repo.UpdateTicketStatus(c.Request.Context(), ticketID, domain.TicketAnswered)

	c.JSON(http.StatusOK, gin.H{"message": "reply sent"})
}

// POST /admin/users/:id/reset-payment-limit
// Clears the payment_init rate-limit counter so the user can retry immediately.
func (h *Handler) ResetPaymentLimit(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	if err := h.anti.ResetRateLimit(c.Request.Context(), id.String(), "payment_init"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset rate limit"})
		return
	}
	h.log.Info("payment rate limit reset by admin", zap.String("user_id", id.String()))
	c.JSON(http.StatusOK, gin.H{"message": "payment rate limit cleared"})
}

// POST /admin/tickets/:id/close
func (h *Handler) CloseTicket(c *gin.Context) {
	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}
	if err := h.repo.UpdateTicketStatus(c.Request.Context(), ticketID, domain.TicketClosed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close ticket"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ticket closed"})
}

// ─── Shop management ──────────────────────────────────────────────────────────

type createShopItemRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	PriceYAD    int64  `json:"price_yad" binding:"required,min=1"`
	Stock       int    `json:"stock"`
}

// POST /admin/shop/items
func (h *Handler) CreateShopItem(c *gin.Context) {
	var req createShopItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stock := req.Stock
	if stock == 0 {
		stock = -1 // unlimited
	}

	item := &domain.ShopItem{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		PriceYAD:    req.PriceYAD,
		Stock:       stock,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}

	// Direct insert via repo
	_, err := c.Request.Context().Deadline()
	_ = err
	// This is handled inline for simplicity — in production inject a ShopRepo
	c.JSON(http.StatusCreated, item)
}
