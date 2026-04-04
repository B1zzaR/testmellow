package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization"})
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := jwtMgr.Parse(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
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
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
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
