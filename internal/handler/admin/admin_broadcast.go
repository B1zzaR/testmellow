package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/vpnplatform/internal/worker"
)

// POST /api/admin/broadcast
// Enqueues a Telegram message for every user that has a linked Telegram account.
// The bot's queue:notify:telegram worker delivers the messages asynchronously.
//
// Per-admin rate-limit (3/hour) prevents accidental or malicious spam — the
// global AdminRateLimit (300/min) on /api/admin/* doesn't differentiate
// cheap GETs from a broadcast that fans out to thousands of Telegram API
// calls and risks a flood-ban on the bot.
func (h *Handler) Broadcast(c *gin.Context) {
	var req struct {
		Message string `json:"message" binding:"required,min=1,max=4096"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	adminIDVal, _ := c.Get("user_id")
	adminID, _ := adminIDVal.(uuid.UUID)
	if err := h.anti.CheckAPIRateLimit(c.Request.Context(), adminID.String(), "admin_broadcast", 3, time.Hour); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	users, err := h.repo.ListUsersWithTelegramID(c.Request.Context())
	if err != nil {
		h.log.Error("broadcast: list users failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load users"})
		return
	}

	count := 0
	for _, u := range users {
		// Use json.Marshal so any unicode / control chars in the admin's
		// message produce valid JSON. The previous fmt.Sprintf("…%q…")
		// path used Go-style quoting which differs from JSON in the corner
		// cases (\xNN escapes, surrogate pairs), and a single such char
		// in the input would silently break every notification after it.
		payload, err := json.Marshal(worker.NotifyTelegramJob{
			TelegramID: u.TelegramID,
			Message:    req.Message,
		})
		if err != nil {
			h.log.Error("broadcast: marshal failed",
				zap.Int64("telegram_id", u.TelegramID), zap.Error(err))
			continue
		}
		if err := h.rdb.LPush(c.Request.Context(), worker.QueueNotifyTelegram, payload).Err(); err != nil {
			h.log.Error("broadcast: push to queue failed",
				zap.Int64("telegram_id", u.TelegramID), zap.Error(err))
			continue
		}
		count++
	}

	h.audit(c, "broadcast", nil, nil, strPtr(fmt.Sprintf("queued %d messages", count)))
	c.JSON(http.StatusOK, gin.H{"queued": count, "total": len(users)})
}
