package middleware

import (
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

// ─── Auth Middleware ──────────────────────────────────────────────────────────

func Auth(jwtMgr *jwtpkg.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "необходима авторизация"})
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := jwtMgr.Parse(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "сессия устарела, войдите снова"})
			return
		}
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextIsAdmin, claims.IsAdmin)
		c.Next()
	}
}

// AdminOnly requires the user to have is_admin = true
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

// CurrentUserID extracts the user ID from context (panics if Auth middleware not used)
func CurrentUserID(c *gin.Context) uuid.UUID {
	val, _ := c.Get(ContextUserID)
	id, _ := val.(uuid.UUID)
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
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
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

// CORS allows requests from the configured allowed origins.
// In production pass the exact frontend origin (e.g. "https://yourdomain.com").
// Pass "*" only for local development.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = struct{}{}
	}
	wildcardAll := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		allowed := wildcardAll
		if !allowed {
			_, allowed = originSet[origin]
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-Id")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == http.MethodOptions {
			if allowed {
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
