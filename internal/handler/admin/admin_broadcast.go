package admin

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// POST /api/admin/broadcast
// Enqueues a Telegram message for every user that has a linked Telegram account.
// The bot's queue:notify:telegram worker delivers the messages asynchronously.
func (h *Handler) Broadcast(c *gin.Context) {
	var req struct {
		Message string `json:"message" binding:"required,min=1,max=4096"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		payload := fmt.Sprintf(`{"telegram_id":%d,"message":%q}`, u.TelegramID, req.Message)
		if err := h.rdb.LPush(c.Request.Context(), "queue:notify:telegram", payload).Err(); err != nil {
			h.log.Error("broadcast: push to queue failed",
				zap.Int64("telegram_id", u.TelegramID), zap.Error(err))
			continue
		}
		count++
	}

	h.audit(c, "broadcast", nil, nil, strPtr(fmt.Sprintf("queued %d messages", count)))
	c.JSON(http.StatusOK, gin.H{"queued": count, "total": len(users)})
}
