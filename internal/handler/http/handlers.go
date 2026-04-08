package httphandler

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/vpnplatform/internal/domain"
	"github.com/vpnplatform/internal/integration/remnawave"
	"github.com/vpnplatform/internal/middleware"
	"github.com/vpnplatform/internal/repository/postgres"
	"github.com/vpnplatform/internal/service"
	jwtpkg "github.com/vpnplatform/pkg/jwt"
	"github.com/vpnplatform/pkg/password"
)

// ─── Auth Handler ─────────────────────────────────────────────────────────────

type AuthHandler struct {
	auth   *service.AuthService
	jwtMgr *jwtpkg.Manager
	rdb    *redis.Client
	log    *zap.Logger
}

func NewAuthHandler(auth *service.AuthService, jwtMgr *jwtpkg.Manager, rdb *redis.Client, log *zap.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, jwtMgr: jwtMgr, rdb: rdb, log: log}
}

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

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
	if !usernameRe.MatchString(req.Username) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "логин может содержать только латинские буквы, цифры и '_'"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	refreshToken, err := h.jwtMgr.GenerateRefresh(user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
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
		// Show the ban message specifically; keep wrong-credentials generic.
		msg := "неверный логин или пароль"
		if strings.Contains(err.Error(), "заблокирован") {
			msg = "ваш аккаунт заблокирован"
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": msg})
		return
	}

	token, err := h.jwtMgr.Generate(user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	refreshToken, err := h.jwtMgr.GenerateRefresh(user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"user_id":       user.ID,
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// POST /api/auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	claims, err := h.jwtMgr.ParseRefresh(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "сессия устарела, войдите снова"})
		return
	}

	// Reject refresh if the user is banned.
	if exists, rErr := h.rdb.Exists(c.Request.Context(), "ban:"+claims.UserID.String()).Result(); rErr == nil && exists > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "аккаунт заблокирован"})
		return
	}

	token, err := h.jwtMgr.Generate(claims.UserID, claims.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}
	newRefresh, err := h.jwtMgr.GenerateRefresh(claims.UserID, claims.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         token,
		"refresh_token": newRefresh,
	})
}

// ─── Profile Handler ──────────────────────────────────────────────────────────

type ProfileHandler struct {
	repo        *postgres.UserRepo
	remna       *remnawave.Client
	rdb         *redis.Client
	botUsername string
	log         *zap.Logger
}

func NewProfileHandler(repo *postgres.UserRepo, remna *remnawave.Client, rdb *redis.Client, botUsername string, log *zap.Logger) *ProfileHandler {
	return &ProfileHandler{repo: repo, remna: remna, rdb: rdb, botUsername: botUsername, log: log}
}

// GET /api/profile
func (h *ProfileHandler) Get(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	user, err := h.repo.GetByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "пользователь не найден"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                  user.ID,
		"username":            user.Username,
		"telegram_id":         user.TelegramID,
		"telegram_username":   user.TelegramUsername,
		"telegram_first_name": user.TelegramFirstName,
		"telegram_last_name":  user.TelegramLastName,
		"telegram_photo_url":  user.TelegramPhotoURL,
		"yad_balance":         user.YADBalance,
		"referral_code":       user.ReferralCode,
		"ltv_kopecks":         user.LTV,
		"trial_used":          user.TrialUsed,
		"is_admin":            user.IsAdmin,
		"is_banned":           user.IsBanned,
		"risk_score":          user.RiskScore,
		"created_at":          user.CreatedAt,
	})
}

// GET /api/profile/connection
// Returns the Remnawave subscribe URL for the authenticated user.
func (h *ProfileHandler) GetConnection(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.CurrentUserID(c)
	user, err := h.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "пользователь не найден"})
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
			c.JSON(http.StatusNotFound, gin.H{"error": "активная подписка не найдена"})
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
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "не удалось настроить VPN-аккаунт, попробуйте позже"})
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
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "не удалось загрузить данные подключения"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subscribe_url": remnaUser.SubscribeURL})
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// POST /api/profile/password
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.repo.GetByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "пользователь не найден"})
		return
	}
	if user.PasswordHash == nil || !password.Verify(*user.PasswordHash, req.OldPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "текущий пароль неверен"})
		return
	}

	hash, err := password.Hash(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}
	if err := h.repo.SetPassword(c.Request.Context(), userID, hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось сменить пароль"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

type updateTelegramRequest struct {
	TelegramID *int64 `json:"telegram_id"`
}

// PUT /api/profile/telegram
func (h *ProfileHandler) UpdateTelegram(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	var req updateTelegramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.TelegramID != nil && *req.TelegramID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный Telegram ID"})
		return
	}

	ctx := c.Request.Context()

	// Unlink case — simple
	if req.TelegramID == nil {
		if err := h.repo.SetTelegramID(ctx, userID, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось отвязать Telegram"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Telegram отвязан", "merged": false})
		return
	}

	// Check whether another account already owns this Telegram ID
	existing, err := h.repo.GetByTelegramID(ctx, *req.TelegramID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	if existing == nil {
		// No conflict — just set the telegram_id
		if err := h.repo.SetTelegramID(ctx, userID, req.TelegramID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось привязать Telegram"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Telegram успешно привязан", "merged": false})
		return
	}

	if existing.ID == userID {
		// Already linked to this account
		c.JSON(http.StatusOK, gin.H{"message": "Telegram уже привязан к этому аккаунту", "merged": false})
		return
	}

	// Snapshot src Remnawave UUID before the merge destroys the src user row
	srcRemnaUUID := ""
	if existing.RemnaUserUUID != nil {
		srcRemnaUUID = *existing.RemnaUserUUID
	}

	// Another account found — merge src (bot account) into dst (this account)
	mergeResult, err := h.repo.MergeAccounts(ctx, userID, existing.ID)
	if err != nil {
		h.log.Error("account merge failed",
			zap.String("dst", userID.String()),
			zap.String("src", existing.ID.String()),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось связать аккаунты"})
		return
	}

	// ── Post-merge Remnawave reconciliation ────────────────────────────────
	// Both accounts may have had active VPN access. Extend dst's expiry to the
	// best remaining active subscription, then disable the orphaned src VPN user.
	dstUser, _ := h.repo.GetByID(ctx, userID)
	if dstUser != nil && dstUser.RemnaUserUUID != nil && *dstUser.RemnaUserUUID != "" {
		dstRemnaUUID := *dstUser.RemnaUserUUID

		// Extend Remnawave expiry to match the longest active sub (now all owned by dst)
		if bestSub, subErr := h.repo.GetActiveSubscription(ctx, userID); subErr == nil && bestSub != nil {
			if extErr := h.remna.UpdateExpiry(ctx, dstRemnaUUID, bestSub.ExpiresAt); extErr != nil {
				h.log.Warn("merge: update remnawave expiry failed", zap.Error(extErr))
			} else {
				h.log.Info("merge: remnawave expiry extended",
					zap.String("remna_uuid", dstRemnaUUID),
					zap.Time("expires_at", bestSub.ExpiresAt),
				)
			}
			// Make sure dst is enabled
			_ = h.remna.EnableUser(ctx, dstRemnaUUID)
		}

		// Disable the src's Remnawave user if it was different (orphaned)
		if srcRemnaUUID != "" && srcRemnaUUID != dstRemnaUUID {
			if disErr := h.remna.DisableUser(ctx, srcRemnaUUID); disErr != nil {
				h.log.Warn("merge: disable src remnawave user failed",
					zap.String("src_remna_uuid", srcRemnaUUID),
					zap.Error(disErr),
				)
			}
		}
	} else if srcRemnaUUID != "" && dstUser != nil {
		// dst had no Remnawave account but src did — the COALESCE in MergeAccounts
		// already copied the src remna_uuid to dst; just make sure it is enabled.
		_ = h.remna.EnableUser(ctx, srcRemnaUUID)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "Аккаунты объединены",
		"merged":               true,
		"transferred_yad":      mergeResult.TransferredYAD,
		"transferred_subs":     mergeResult.TransferredSubs,
		"transferred_payments": mergeResult.TransferredPayments,
	})
}

// GET /api/profile/traffic
// Returns traffic usage stats for the authenticated user from Remnawave.
func (h *ProfileHandler) GetTraffic(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.CurrentUserID(c)
	user, err := h.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "пользователь не найден"})
		return
	}
	if user.RemnaUserUUID == nil || *user.RemnaUserUUID == "" {
		c.JSON(http.StatusOK, gin.H{"used_bytes": 0, "limit_bytes": 0})
		return
	}
	remnaUser, err := h.remna.GetUser(ctx, *user.RemnaUserUUID)
	if err != nil {
		h.log.Warn("remnawave get user for traffic", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{"used_bytes": 0, "limit_bytes": 0})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"used_bytes":  remnaUser.UsedTrafficBytes,
		"limit_bytes": remnaUser.TrafficLimitBytes,
	})
}

// linkCodeChars — unambiguous alphanumeric characters for link codes.
const linkCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func generateLinkCode() (string, error) {
	code := make([]byte, 6)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(linkCodeChars))))
		if err != nil {
			return "", err
		}
		code[i] = linkCodeChars[n.Int64()]
	}
	return string(code), nil
}

// POST /api/profile/telegram/link-code
// Generates a short-lived one-time code the user can send to the Telegram bot
// to link their web account without knowing their numeric Telegram ID.
func (h *ProfileHandler) GenerateLinkCode(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.CurrentUserID(c)

	code, err := generateLinkCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	key := fmt.Sprintf("tg:link:%s", code)
	if err := h.rdb.Set(ctx, key, userID.String(), 5*time.Minute).Err(); err != nil {
		h.log.Error("generate link code: redis set", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":         code,
		"bot_username": h.botUsername,
		"expires_in":   300,
	})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "пользователь не найден"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось загрузить историю"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось загрузить подписки"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subscriptions": subs})
}

// GET /api/subscriptions/:id
func (h *SubscriptionHandler) GetByID(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	subID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный идентификатор подписки"})
		return
	}

	subs, err := h.svc.GetUserSubscriptions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось загрузить подписки"})
		return
	}
	for _, s := range subs {
		if s.ID == subID {
			c.JSON(http.StatusOK, s)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "подписка не найдена"})
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
	svc  *service.SubscriptionService
	repo *postgres.UserRepo
	log  *zap.Logger
}

func NewPaymentHandler(svc *service.SubscriptionService, repo *postgres.UserRepo, log *zap.Logger) *PaymentHandler {
	return &PaymentHandler{svc: svc, repo: repo, log: log}
}

// GET /api/payments/pending
func (h *PaymentHandler) ListPending(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	payments, err := h.svc.GetPendingPayments(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось загрузить платежи"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный идентификатор платежа"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный идентификатор платежа"})
		return
	}
	payment, err := h.svc.CheckPaymentStatus(c.Request.Context(), userID, paymentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, payment)
}

// GET /api/payments/history?page=1&per_page=10
func (h *PaymentHandler) ListHistory(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}
	offset := (page - 1) * perPage
	payments, total, err := h.repo.GetPaymentHistory(c.Request.Context(), userID, perPage, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось загрузить историю платежей"})
		return
	}
	if payments == nil {
		payments = []*domain.Payment{}
	}
	c.JSON(http.StatusOK, gin.H{"payments": payments, "total": total})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось загрузить рефералы"})
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
	eco  *service.EconomyService
	repo *postgres.UserRepo
	log  *zap.Logger
}

func NewPromoHandler(eco *service.EconomyService, repo *postgres.UserRepo, log *zap.Logger) *PromoHandler {
	return &PromoHandler{eco: eco, repo: repo, log: log}
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
		"message":          "Promo code applied",
		"promo_type":       promo.PromoType,
		"yad_earned":       promo.YADAmount,
		"discount_percent": promo.DiscountPercent,
	})
}

// GET /api/promo/discount/active
func (h *PromoHandler) ActiveDiscount(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	code, percent, err := h.repo.GetActiveDiscount(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось получить информацию о скидке"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"active_discount_code":    code,
		"active_discount_percent": percent,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось создать обращение"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось отправить сообщение"})
		return
	}

	c.JSON(http.StatusCreated, ticket)
}

// GET /api/tickets
func (h *TicketHandler) List(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	tickets, err := h.repo.ListTickets(c.Request.Context(), &userID, "", 20, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось загрузить обращения"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tickets": tickets})
}

// GET /api/tickets/:id
func (h *TicketHandler) GetWithMessages(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный идентификатор обращения"})
		return
	}

	ticket, err := h.repo.GetTicketByID(c.Request.Context(), ticketID)
	if err != nil || ticket == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "обращение не найдено"})
		return
	}
	// Access control: user can only see their own tickets
	if ticket.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ запрещён"})
		return
	}

	msgs, err := h.repo.GetTicketMessages(c.Request.Context(), ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось загрузить сообщения"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный идентификатор обращения"})
		return
	}

	var req replyTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ticket, err := h.repo.GetTicketByID(c.Request.Context(), ticketID)
	if err != nil || ticket == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "обращение не найдено"})
		return
	}
	if ticket.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ запрещён"})
		return
	}
	if ticket.Status == domain.TicketClosed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "обращение закрыто"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось отправить сообщение"})
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

type buySubscriptionWithYADRequest struct {
	Plan string `json:"plan" binding:"required"`
}

// POST /api/shop/buy-subscription
func (h *ShopHandler) BuySubscription(c *gin.Context) {
	var req buySubscriptionWithYADRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plan := domain.SubscriptionPlan(req.Plan)
	if domain.PlanYADPrice(plan) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan"})
		return
	}

	userID := middleware.CurrentUserID(c)
	sub, err := h.eco.BuySubscriptionWithYAD(c.Request.Context(), userID, plan)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Subscription activated",
		"expires_at": sub.ExpiresAt,
		"plan":       sub.Plan,
	})
}

// ─── Device Handler ───────────────────────────────────────────────────────────

type DeviceHandler struct {
	svc *service.DeviceService
	log *zap.Logger
}

func NewDeviceHandler(svc *service.DeviceService, log *zap.Logger) *DeviceHandler {
	return &DeviceHandler{svc: svc, log: log}
}

// GET /api/devices
func (h *DeviceHandler) List(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	devices, err := h.svc.ListDevices(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
		return
	}

	type deviceResponse struct {
		ID         uuid.UUID `json:"id"`
		DeviceName string    `json:"device_name"`
		LastActive string    `json:"last_active"`
		IsActive   bool      `json:"is_active"`
		IsInactive bool      `json:"is_inactive"`
	}

	result := make([]deviceResponse, 0, len(devices))
	activeCount := 0
	for _, d := range devices {
		result = append(result, deviceResponse{
			ID:         d.ID,
			DeviceName: d.DeviceName,
			LastActive: d.LastActive.Format(time.RFC3339),
			IsActive:   d.IsActive,
			IsInactive: d.IsInactive(),
		})
		if d.IsActive {
			activeCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"devices": result,
		"count":   activeCount,
		"limit":   domain.DeviceMaxPerUser,
	})
}

// POST /api/devices/:id/disconnect
func (h *DeviceHandler) Disconnect(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	deviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный ID устройства"})
		return
	}

	if err := h.svc.DisconnectDevice(c.Request.Context(), userID, deviceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Устройство отключено"})
}
