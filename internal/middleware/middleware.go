package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	redisrepo "github.com/vpnplatform/internal/repository/redis"
	jwtpkg "github.com/vpnplatform/pkg/jwt"
)

const (
	ContextUserID  = "user_id"
	ContextIsAdmin = "is_admin"
)

// ─── Body Size Limit ──────────────────────────────────────────────────────────

// MaxBodySize rejects requests whose body exceeds maxBytes (default: 1 MiB).
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
		if c.Request.Body != nil {
			// Check if the body was too large (MaxBytesReader sets error on read)
			_ = c.Request.Body.Close()
		}
	}
}

// ─── Auth Middleware ──────────────────────────────────────────────────────────

// Auth reads a JWT from the Authorization: Bearer header or the access_token
// HttpOnly cookie (H-7: cookie is preferred as it is not XSS-accessible).
// rdb is used to reject tokens issued before a password change (session invalidation).
func Auth(jwtMgr *jwtpkg.Manager, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string

		// Prefer the Authorization header (keeps CLI / mobile clients working).
		if header := c.GetHeader("Authorization"); strings.HasPrefix(header, "Bearer ") {
			tokenStr = strings.TrimPrefix(header, "Bearer ")
		} else if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
			tokenStr = cookie
		}

		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "необходима авторизация"})
			return
		}
		claims, err := jwtMgr.Parse(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "сессия устарела, войдите снова"})
			return
		}

		// Reject tokens issued before the user last changed their password.
		if claims.IssuedAt != nil {
			if err := redisrepo.CheckPasswordVersion(c.Request.Context(), rdb, claims.UserID.String(), claims.IssuedAt.Time); err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "сессия устарела, войдите снова"})
				return
			}
		}

		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextIsAdmin, claims.IsAdmin)
		c.Next()
	}
}

// AdminOnly requires the user to have is_admin = true (JWT claim check only).
// For admin API routes prefer AdminDBCheck which re-validates against the DB (H-4).
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, _ := c.Get(ContextIsAdmin)
		if isAdmin != true {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "доступ запрещён"})
			return
		}
		c.Next()
	}
}

// AdminDBCheck re-verifies the admin flag from the database on every request
// so that a demoted admin cannot continue using a previously-issued JWT (H-4).
// isAdminFn should be a lightweight closure over the user repository:
//
//	func(ctx context.Context, id uuid.UUID) (bool, error) { return repo.IsAdmin(ctx, id) }
func AdminDBCheck(isAdminFn func(ctx context.Context, id uuid.UUID) (bool, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First gate: fast JWT claim check (avoids DB hit for non-admin tokens).
		isAdminClaim, _ := c.Get(ContextIsAdmin)
		if isAdminClaim != true {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "доступ запрещён"})
			return
		}
		userID := CurrentUserID(c)
		if userID == (uuid.UUID{}) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "доступ запрещён"})
			return
		}
		ok, err := isAdminFn(c.Request.Context(), userID)
		if err != nil || !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "доступ запрещён"})
			return
		}
		c.Next()
	}
}

// CurrentUserID extracts the user ID from context. It is only safe to call
// from a handler that runs after Auth middleware. If Auth is missing (route
// misconfig), this returns the nil UUID and aborts the request with 401 to
// prevent the handler from silently querying user_id = '0000…' and leaking
// data when filters happen to match an absent user.
func CurrentUserID(c *gin.Context) uuid.UUID {
	val, ok := c.Get(ContextUserID)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "необходима авторизация"})
		return uuid.UUID{}
	}
	id, ok := val.(uuid.UUID)
	if !ok || id == (uuid.UUID{}) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "необходима авторизация"})
		return uuid.UUID{}
	}
	return id
}

// ─── Request ID Middleware ────────────────────────────────────────────────────

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := uuid.New().String()
		c.Set("request_id", id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}

// ─── Structured Logger Middleware ─────────────────────────────────────────────

func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		// Skip noisy browser-generated paths
		if path == "/favicon.ico" {
			return
		}

		log.Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
			zap.String("request_id", c.GetString("request_id")),
		)
	}
}

// ─── Security Headers ─────────────────────────────────────────────────────────

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		// X-XSS-Protection removed: deprecated header that browsers either
		// ignore (Firefox, Safari, modern Chrome) or implemented with their
		// own bugs. CSP supersedes it.
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=(), interest-cohort=()")
		c.Next()
	}
}

// ─── Recovery Middleware ──────────────────────────────────────────────────────

func Recovery(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered",
					zap.Any("error", r),
					zap.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}

// ─── CORS Middleware ──────────────────────────────────────────────────────────

// CORS allows requests from the configured allow-list of origins. Wildcard
// '*' is special: it is permitted only in dev (where the entire origin set
// is literally ["*"]) and in that mode credentials are NOT sent — that's the
// only configuration the spec accepts.
//
// In production, the previous "echo any Origin and set Allow-Credentials:
// true" path was effectively wildcard-with-credentials and was a textbook
// cross-origin data exfiltration vector. config.Load() now rejects '*' in
// production, so reaching this branch with credentials means dev only.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o != "" && o != "*" {
			originSet[o] = struct{}{}
		}
	}
	devWildcard := len(allowedOrigins) == 1 && strings.TrimSpace(allowedOrigins[0]) == "*"

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		_, allowed := originSet[origin]

		switch {
		case allowed:
			// Whitelisted: echo back the (validated) origin and allow credentials.
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-Id")
			c.Header("Access-Control-Max-Age", "86400")
		case devWildcard:
			// Dev-only: send the literal '*' WITHOUT credentials (spec-compliant).
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-Id")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == http.MethodOptions {
			if allowed || devWildcard {
				c.AbortWithStatus(http.StatusNoContent)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}

		c.Next()
	}
}

// ─── Admin Rate Limit Middleware ──────────────────────────────────────────────

// AdminRateLimit limits each authenticated admin to maxReq requests per window
// on the admin API. Uses Redis for global consistency across instances.
func AdminRateLimit(rdb *redis.Client, maxReq int64, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get(ContextUserID)
		uid, ok := userID.(uuid.UUID)
		if !ok {
			c.Next()
			return
		}
		key := "rl:admin:" + uid.String()
		count, err := redisrepo.Increment(c.Request.Context(), rdb, key, window)
		if err != nil {
			// Fail open — don't block admins on Redis errors
			c.Next()
			return
		}
		if count > maxReq {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

// ─── Banned User Check ────────────────────────────────────────────────────────

// BannedCheck aborts requests from users whose ID appears in the Redis ban set.
// Must run AFTER the Auth middleware (requires ContextUserID to be set).
// Ban key format: "ban:<userID>" (set by admin BanUser handler, TTL = 30 days).
func BannedCheck(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get(ContextUserID)
		uid, ok := userID.(uuid.UUID)
		if !ok {
			c.Next()
			return
		}
		exists, err := rdb.Exists(c.Request.Context(), "ban:"+uid.String()).Result()
		if err != nil {
			// Fail open on Redis error — better to allow than to block everyone.
			c.Next()
			return
		}
		if exists > 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "аккаунт заблокирован"})
			return
		}
		c.Next()
	}
}

// SetBanKey writes (or removes) the Redis ban key for a user.
// Call duration = 30 days so the key auto-expires even if unban is missed.
// Exported so admin handlers can call it without importing middleware.
func SetBanKey(ctx context.Context, rdb *redis.Client, userID uuid.UUID, banned bool) error {
	key := "ban:" + userID.String()
	if banned {
		return rdb.Set(ctx, key, 1, 30*24*time.Hour).Err()
	}
	return rdb.Del(ctx, key).Err()
}

// IPRateLimit limits each client IP to maxReq requests per window.
// Used for sensitive unauthenticated endpoints (login, register) and promo code application.
func IPRateLimit(rdb *redis.Client, prefix string, maxReq int64, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("rl:%s:%s", prefix, c.ClientIP())
		count, err := redisrepo.Increment(c.Request.Context(), rdb, key, window)
		if err != nil {
			// Fail open on Redis errors
			c.Next()
			return
		}
		if count > maxReq {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "слишком много запросов, повторите позже"})
			return
		}
		c.Next()
	}
}

// UserRateLimit limits each authenticated user to maxReq requests per window.
// Uses the user_id from JWT context so it works across IPs (e.g. mobile + desktop).
func UserRateLimit(rdb *redis.Client, maxReq int64, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get(ContextUserID)
		uid, ok := userID.(uuid.UUID)
		if !ok {
			c.Next()
			return
		}
		key := "rl:user:" + uid.String()
		count, err := redisrepo.Increment(c.Request.Context(), rdb, key, window)
		if err != nil {
			// Fail open on Redis errors
			c.Next()
			return
		}
		if count > maxReq {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "слишком много запросов, повторите позже"})
			return
		}
		c.Next()
	}
}
