package httphandler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"sort"
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
	redisrepo "github.com/vpnplatform/internal/repository/redis"
	"github.com/vpnplatform/internal/service"
	"github.com/vpnplatform/internal/worker"
	jwtpkg "github.com/vpnplatform/pkg/jwt"
	"github.com/vpnplatform/pkg/password"
)

// ─── Auth Handler ─────────────────────────────────────────────────────────────

type AuthHandler struct {
	auth   *service.AuthService
	repo   *postgres.UserRepo
	jwtMgr *jwtpkg.Manager
	rdb    *redis.Client
	log    *zap.Logger
}

func NewAuthHandler(auth *service.AuthService, repo *postgres.UserRepo, jwtMgr *jwtpkg.Manager, rdb *redis.Client, log *zap.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, repo: repo, jwtMgr: jwtMgr, rdb: rdb, log: log}
}

// setAuthCookies writes access and refresh tokens as HttpOnly cookies (H-7).
// The refresh cookie is scoped to /api/auth/refresh to prevent inadvertent
// transmission. The Secure flag is set in production mode.
func setAuthCookies(c *gin.Context, token, refresh string) {
	secure := gin.Mode() == gin.ReleaseMode
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("access_token", token, int((24 * time.Hour).Seconds()), "/", "", secure, true)
	c.SetCookie("refresh_token", refresh, int((30 * 24 * time.Hour).Seconds()), "/api/auth/refresh", "", secure, true)
}

// clearAuthCookies expires both auth cookies (for logout).
func clearAuthCookies(c *gin.Context) {
	secure := gin.Mode() == gin.ReleaseMode
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("access_token", "", -1, "/", "", secure, true)
	c.SetCookie("refresh_token", "", -1, "/api/auth/refresh", "", secure, true)
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

	refreshToken, jti, err := h.jwtMgr.GenerateRefresh(user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	// H-8: register refresh token JTI in allowlist so it can be revoked.
	if err := redisrepo.RegisterRefreshToken(c.Request.Context(), h.rdb, jti, user.ID.String(), h.jwtMgr.RefreshTTL()); err != nil {
		h.log.Warn("register refresh token failed", zap.Error(err))
	}

	// H-7: deliver tokens via HttpOnly cookies, not in response body.
	setAuthCookies(c, token, refreshToken)

	c.JSON(http.StatusCreated, gin.H{
		"user_id":       user.ID,
		"is_admin":      user.IsAdmin,
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
		Username:  req.Username,
		Password:  req.Password,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
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

	// ── 2FA via Telegram ──────────────────────────────────────────────────
	if user.TFAEnabled && user.TelegramID != nil && *user.TelegramID != 0 {
		challengeID, err := generateChallengeID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
			return
		}
		if err := redisrepo.Create2FAChallenge(c.Request.Context(), h.rdb, challengeID, user.ID.String()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
			return
		}
		// Send Telegram notification via worker queue
		ip := c.ClientIP()
		ua := c.GetHeader("User-Agent")
		msg := fmt.Sprintf(
			"🔐 <b>Запрос на вход</b>\n\nIP: %s\nUA: %.60s\n\nПодтвердите вход в аккаунт.",
			ip, ua,
		)
		_ = worker.Enqueue(c.Request.Context(), h.rdb, worker.QueueTFAChallenge, worker.TFAChallengeJob{
			TelegramID:  *user.TelegramID,
			ChallengeID: challengeID,
			Message:     msg,
		})
		c.JSON(http.StatusOK, gin.H{
			"tfa_required": true,
			"challenge_id": challengeID,
		})
		return
	}

	token, err := h.jwtMgr.Generate(user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	refreshToken, jti, err := h.jwtMgr.GenerateRefresh(user.ID, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	// H-8: register refresh token JTI in allowlist.
	if err := redisrepo.RegisterRefreshToken(c.Request.Context(), h.rdb, jti, user.ID.String(), h.jwtMgr.RefreshTTL()); err != nil {
		h.log.Warn("register refresh token failed", zap.Error(err))
	}

	// H-7: deliver tokens via HttpOnly cookies.
	setAuthCookies(c, token, refreshToken)

	c.JSON(http.StatusOK, gin.H{
		"user_id":  user.ID,
		"is_admin": user.IsAdmin,
	})
}

// POST /api/auth/refresh
// Reads the refresh token from the HttpOnly cookie (H-7) or JSON body (backward compat).
// Validates the JTI against the revocation allowlist (H-8) and rotates both tokens.
func (h *AuthHandler) Refresh(c *gin.Context) {
	// Accept token from cookie first, then fall back to JSON body.
	var refreshTokenStr string
	if cookie, err := c.Cookie("refresh_token"); err == nil && cookie != "" {
		refreshTokenStr = cookie
	} else {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = c.ShouldBindJSON(&req)
		refreshTokenStr = req.RefreshToken
	}
	if refreshTokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "сессия устарела, войдите снова"})
		return
	}

	claims, err := h.jwtMgr.ParseRefresh(refreshTokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "сессия устарела, войдите снова"})
		return
	}

	// H-8: validate JTI against the allowlist and delete it atomically (one-use).
	jti := claims.ID
	if _, err := redisrepo.ValidateAndRevokeRefreshToken(c.Request.Context(), h.rdb, jti); err != nil {
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
	newRefresh, newJTI, err := h.jwtMgr.GenerateRefresh(claims.UserID, claims.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	// Register the new JTI in the allowlist.
	if err := redisrepo.RegisterRefreshToken(c.Request.Context(), h.rdb, newJTI, claims.UserID.String(), h.jwtMgr.RefreshTTL()); err != nil {
		h.log.Warn("register new refresh token failed", zap.Error(err))
	}

	// H-7: deliver new tokens via HttpOnly cookies.
	setAuthCookies(c, token, newRefresh)

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// POST /api/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	// Clear auth cookies on client
	clearAuthCookies(c)
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// GET /api/auth/2fa/check?challenge_id=...
// Public endpoint — polled by the frontend after login returns tfa_required=true.
func (h *AuthHandler) TFACheck(c *gin.Context) {
	challengeID := c.Query("challenge_id")
	if challengeID == "" || len(challengeID) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid challenge_id"})
		return
	}

	userID, status, err := redisrepo.Get2FAChallenge(c.Request.Context(), h.rdb, challengeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
		return
	}
	if userID == "" {
		c.JSON(http.StatusGone, gin.H{"status": "expired"})
		return
	}

	switch status {
	case redisrepo.TFAApproved:
		// Issue tokens
		uid, err := uuid.Parse(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
			return
		}
		user, err := h.repo.GetByID(c.Request.Context(), uid)
		if err != nil || user == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
			return
		}
		token, err := h.jwtMgr.Generate(uid, user.IsAdmin)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
			return
		}
		refreshToken, jti, err := h.jwtMgr.GenerateRefresh(uid, user.IsAdmin)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
			return
		}
		if err := redisrepo.RegisterRefreshToken(c.Request.Context(), h.rdb, jti, userID, h.jwtMgr.RefreshTTL()); err != nil {
			h.log.Warn("register refresh token failed (2FA)", zap.Error(err))
		}
		setAuthCookies(c, token, refreshToken)
		c.JSON(http.StatusOK, gin.H{
			"status":   "approved",
			"user_id":  uid,
			"is_admin": user.IsAdmin,
		})
	case redisrepo.TFADenied:
		c.JSON(http.StatusOK, gin.H{"status": "denied"})
	default:
		c.JSON(http.StatusOK, gin.H{"status": "pending"})
	}
}

func generateChallengeID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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
		"tfa_enabled":         user.TFAEnabled,
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
			remnaName := user.RemnaUsername()
			remnaUser, lookupErr := h.remna.GetUserByUsername(ctx, remnaName)
			if lookupErr != nil || remnaUser == nil || remnaUser.UUID == "" {
				// Legacy fallback: try UUID-based username from older registrations.
				remnaUser, lookupErr = h.remna.GetUserByUsername(ctx, userID.String())
			}
			if lookupErr == nil && remnaUser != nil && remnaUser.UUID != "" {
				remnaUUID = remnaUser.UUID
				_ = h.repo.UpdateRemnaUUID(ctx, userID, remnaUUID)
				c.JSON(http.StatusOK, gin.H{"subscribe_url": remnaUser.SubscribeURL})
				return
			}
			// Path 3: create a new Remnawave account
			remnaUser, createErr := h.remna.CreateUser(ctx, remnaName, subs.ExpiresAt)
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

	// Invalidate all tokens issued before this moment so an attacker who obtained
	// the old password (or a leaked token) cannot continue using the session.
	if vErr := redisrepo.SetPasswordVersion(c.Request.Context(), h.rdb, userID.String(), time.Now()); vErr != nil {
		h.log.Warn("set password version failed", zap.Error(vErr))
	}

	// Activity log (best-effort)
	ip := c.ClientIP()
	ua := strings.TrimSpace(c.GetHeader("User-Agent"))
	var ipPtr *string
	var uaPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	if ua != "" {
		uaPtr = &ua
	}
	_ = h.repo.CreateAccountActivity(c.Request.Context(), &domain.AccountActivity{
		ID:        uuid.New(),
		UserID:    userID,
		EventType: "password_change",
		IP:        ipPtr,
		UserAgent: uaPtr,
		Details:   nil,
		CreatedAt: time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

// PUT /api/profile/telegram
// This endpoint only allows UNLINKING (sending null telegram_id).
// Linking is performed exclusively via the Telegram bot /link CODE flow (C-1).
// Unlinking requires a short-lived confirmation code delivered via Telegram.
func (h *ProfileHandler) UpdateTelegram(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	var req struct {
		TelegramID *int64 `json:"telegram_id"`
		Code       string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Reject any attempt to set an arbitrary telegram_id via the API.
	// Telegram linking must go through the verified bot flow.
	if req.TelegramID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "привязка Telegram возможна только через бот"})
		return
	}

	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "для отвязки Telegram нужен код подтверждения из бота"})
		return
	}

	user, err := h.repo.GetByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "пользователь не найден"})
		return
	}
	if user.TelegramID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram уже не привязан"})
		return
	}

	// Validate one-time unlink code stored in Redis (GetDel to consume).
	key := fmt.Sprintf("tg:unlink:%s", code)
	uid, rErr := h.rdb.GetDel(c.Request.Context(), key).Result()
	if rErr == redis.Nil || uid == "" || uid != userID.String() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "код не найден или истёк"})
		return
	}
	if rErr != nil {
		h.log.Warn("unlink code redis getdel", zap.Error(rErr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	if err := h.repo.SetTelegramID(c.Request.Context(), userID, nil); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось отвязать Telegram"})
		return
	}

	// Auto-disable 2FA when Telegram is unlinked
	_ = h.repo.SetTFAEnabled(c.Request.Context(), userID, false)

	// Activity log (best-effort)
	ip := c.ClientIP()
	ua := strings.TrimSpace(c.GetHeader("User-Agent"))
	var ipPtr *string
	var uaPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	if ua != "" {
		uaPtr = &ua
	}
	_ = h.repo.CreateAccountActivity(c.Request.Context(), &domain.AccountActivity{
		ID:        uuid.New(),
		UserID:    userID,
		EventType: "telegram_unlink",
		IP:        ipPtr,
		UserAgent: uaPtr,
		Details:   nil,
		CreatedAt: time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "Telegram отвязан"})
}

// POST /api/profile/telegram/unlink-code
// Generates a short-lived one-time code and sends it to the linked Telegram.
func (h *ProfileHandler) GenerateUnlinkCode(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.CurrentUserID(c)

	user, err := h.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "пользователь не найден"})
		return
	}
	if user.TelegramID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram не привязан"})
		return
	}

	// Rate-limit: 3 requests per 10 minutes per user.
	rlKey := fmt.Sprintf("rl:unlink_code:%s", userID.String())
	count, rlErr := redisrepo.Increment(ctx, h.rdb, rlKey, 10*time.Minute)
	if rlErr == nil && count > 3 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "слишком много запросов, повторите позже"})
		return
	}

	code, err := generateLinkCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}
	key := fmt.Sprintf("tg:unlink:%s", code)
	if err := h.rdb.Set(ctx, key, userID.String(), 10*time.Minute).Err(); err != nil {
		h.log.Error("generate unlink code: redis set", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера, попробуйте позже"})
		return
	}

	// Send code to Telegram via worker queue (if configured).
	_ = worker.Enqueue(ctx, h.rdb, worker.QueueNotifyTelegram, worker.NotifyTelegramJob{
		TelegramID: *user.TelegramID,
		Message:    fmt.Sprintf("🔐 <b>Код для отвязки Telegram</b>\n\nКод: <code>%s</code>\n\nСрок действия: 10 минут.", code),
	})

	c.JSON(http.StatusOK, gin.H{"message": "код отправлен в Telegram", "expires_in": 600})
}

// GET /api/profile/activity?limit=50
func (h *ProfileHandler) Activity(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, err := h.repo.ListAccountActivity(c.Request.Context(), userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось загрузить активность"})
		return
	}
	if items == nil {
		items = []*domain.AccountActivity{}
	}
	c.JSON(http.StatusOK, gin.H{"activity": items})
}

// POST /api/profile/tfa
// Toggle 2FA via Telegram. Requires Telegram to be linked.
func (h *ProfileHandler) ToggleTFA(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	userID := middleware.CurrentUserID(c)
	user, err := h.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "пользователь не найден"})
		return
	}

	if req.Enabled && (user.TelegramID == nil || *user.TelegramID == 0) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "сначала привяжите Telegram"})
		return
	}

	if err := h.repo.SetTFAEnabled(ctx, userID, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tfa_enabled": req.Enabled})
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
		"used_bytes":  remnaUser.UserTraffic.UsedTrafficBytes,
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
// Rate-limited to 3 requests per 5 minutes per user (M-2).
func (h *ProfileHandler) GenerateLinkCode(c *gin.Context) {
	ctx := c.Request.Context()
	userID := middleware.CurrentUserID(c)

	// M-2: rate-limit to prevent code-farming / bot spamming.
	rlKey := fmt.Sprintf("rl:link_code:%s", userID.String())
	count, rlErr := redisrepo.Increment(ctx, h.rdb, rlKey, 5*time.Minute)
	if rlErr == nil && count > 3 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "слишком много запросов, повторите позже"})
		return
	}

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
	svc  *service.SubscriptionService
	repo *postgres.UserRepo
	log  *zap.Logger
}

func NewSubscriptionHandler(svc *service.SubscriptionService, repo *postgres.UserRepo, log *zap.Logger) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc, repo: repo, log: log}
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

	// Check if real money purchases are blocked
	settings, err := h.repo.GetPlatformSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка server config"})
		return
	}
	if settings != nil && settings.BlockRealMoneyPurchases {
		c.JSON(http.StatusForbidden, gin.H{"error": "Пока что покупки заблокированы администратором."})
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

	// Check if real money purchases are blocked
	settings, err := h.repo.GetPlatformSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка server config"})
		return
	}
	if settings != nil && settings.BlockRealMoneyPurchases {
		c.JSON(http.StatusForbidden, gin.H{"error": "Пока что покупки заблокированы администратором."})
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

// POST /api/payments/:id/touch — refreshes expires_at to now+30 minutes when
// the user returns to the app after visiting the Platega payment page.
func (h *PaymentHandler) Touch(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный идентификатор платежа"})
		return
	}
	payment, err := h.svc.TouchPayment(c.Request.Context(), userID, paymentID)
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
	Message string `json:"message" binding:"required,min=1,max=4096"`
}

// POST /api/tickets
func (h *TicketHandler) Create(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	var req createTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Limit: 1 open ticket per user.
	openCount, err := h.repo.CountOpenTickets(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
		return
	}
	if openCount > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "У вас уже есть открытый тикет. Дождитесь его закрытия или закройте вручную."})
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
	Message string `json:"message" binding:"required,min=1,max=4096"`
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
	repo   *postgres.UserRepo
	eco    *service.EconomyService
	subSvc *service.SubscriptionService
	log    *zap.Logger
}

func NewShopHandler(repo *postgres.UserRepo, eco *service.EconomyService, subSvc *service.SubscriptionService, log *zap.Logger) *ShopHandler {
	return &ShopHandler{repo: repo, eco: eco, subSvc: subSvc, log: log}
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
	Quantity int    `json:"quantity" binding:"required,min=1,max=100"`
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

// POST /api/shop/buy-device-expansion  (YAD payment — adds +1 or +3 devices)
func (h *ShopHandler) BuyDeviceExpansion(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	expansion, err := h.eco.BuyDeviceExpansion(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Расширение устройств активировано",
		"extra_devices": expansion.ExtraDevices,
		"expires_at":    expansion.ExpiresAt,
		"total_limit":   domain.DeviceMaxPerUser + expansion.ExtraDevices,
	})
}

type buyDeviceExpansionMoneyRequest struct {
	ReturnURL string `json:"return_url"`
}

// POST /api/shop/buy-device-expansion-money  (Platega payment — adds +1 device)
func (h *ShopHandler) BuyDeviceExpansionMoney(c *gin.Context) {
	var req buyDeviceExpansionMoneyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.CurrentUserID(c)
	redirect, payment, err := h.subSvc.InitiateDeviceExpansionPayment(c.Request.Context(), userID, req.ReturnURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"payment_id":   payment.ID,
		"redirect_url": redirect,
		"amount_rub":   float64(payment.AmountKopecks) / 100,
		"expires_in":   "15 minutes",
	})
}

// POST /api/shop/extend-device-expansion  (YAD payment — extends expansion expiry)
func (h *ShopHandler) ExtendDeviceExpansion(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	expansion, err := h.eco.ExtendDeviceExpansionYAD(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Расширение устройств продлено",
		"extra_devices": expansion.ExtraDevices,
		"expires_at":    expansion.ExpiresAt,
		"total_limit":   domain.DeviceMaxPerUser + expansion.ExtraDevices,
	})
}

// POST /api/shop/extend-device-expansion-money  (Platega payment — extends expansion expiry)
func (h *ShopHandler) ExtendDeviceExpansionMoney(c *gin.Context) {
	var req buyDeviceExpansionMoneyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.CurrentUserID(c)
	redirect, payment, err := h.subSvc.InitiateDeviceExpansionExtendPayment(c.Request.Context(), userID, req.ReturnURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"payment_id":   payment.ID,
		"redirect_url": redirect,
		"amount_rub":   float64(payment.AmountKopecks) / 100,
		"expires_in":   "15 minutes",
	})
}

// ─── Device Handler ───────────────────────────────────────────────────────────

type DeviceHandler struct {
	svc  *service.DeviceService
	repo *postgres.UserRepo
	log  *zap.Logger
}

func NewDeviceHandler(svc *service.DeviceService, repo *postgres.UserRepo, log *zap.Logger) *DeviceHandler {
	return &DeviceHandler{svc: svc, repo: repo, log: log}
}

// GET /api/devices
func (h *DeviceHandler) List(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	devices, err := h.svc.ListDevices(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ошибка сервера"})
		return
	}

	// Dynamic device limit based on active expansion
	limit, _ := h.repo.GetEffectiveDeviceLimit(c.Request.Context(), userID)

	// Get expansion info for the response
	expansion, _ := h.repo.GetActiveDeviceExpansion(c.Request.Context(), userID)

	type deviceResponse struct {
		ID             string `json:"id"`
		DeviceName     string `json:"device_name"`
		LastActive     string `json:"last_active"`
		IsActive       bool   `json:"is_active"`
		IsInactive     bool   `json:"is_inactive"`
		CanDeleteAfter string `json:"can_delete_after"`
		IsBlocked      bool   `json:"is_blocked"`
	}

	// Sort devices by creation time to determine blocking order.
	// Oldest devices get priority (unblocked), newest get blocked when over limit.
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].CreatedAt.Before(devices[j].CreatedAt)
	})

	result := make([]deviceResponse, 0, len(devices))
	activeCount := 0
	for idx, d := range devices {
		id := d.HwidID
		if id == "" {
			id = d.ID.String()
		}
		canDeleteAfter := d.LastActive.Add(time.Duration(domain.DeviceInactiveDays) * 24 * time.Hour)
		blocked := idx >= limit
		result = append(result, deviceResponse{
			ID:             id,
			DeviceName:     d.DeviceName,
			LastActive:     d.LastActive.Format(time.RFC3339),
			IsActive:       d.IsActive,
			IsInactive:     d.IsInactive(),
			CanDeleteAfter: canDeleteAfter.Format(time.RFC3339),
			IsBlocked:      blocked,
		})
		if d.IsActive && !blocked {
			activeCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"devices":   result,
		"count":     activeCount,
		"limit":     limit,
		"expansion": expansion,
	})
}

// POST /api/devices/:id/disconnect
func (h *DeviceHandler) Disconnect(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	hwidID := c.Param("id")
	if hwidID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный ID устройства"})
		return
	}

	if err := h.svc.DisconnectDevice(c.Request.Context(), userID, hwidID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Устройство отключено"})
}
