package httphandler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vpnplatform/internal/middleware"
	"github.com/vpnplatform/internal/repository/postgres"
	redisrepo "github.com/vpnplatform/internal/repository/redis"
)

// SuggestionHandler handles anonymous suggestions submitted from the user cabinet.
type SuggestionHandler struct {
	repo *postgres.UserRepo
	rdb  *redis.Client
	log  *zap.Logger
}

func NewSuggestionHandler(repo *postgres.UserRepo, rdb *redis.Client, log *zap.Logger) *SuggestionHandler {
	return &SuggestionHandler{repo: repo, rdb: rdb, log: log}
}

// POST /api/suggestions
// Accepts an anonymous text suggestion. The authenticated user identity is
// intentionally NOT stored — only the message body is saved.
func (h *SuggestionHandler) Submit(c *gin.Context) {
	userID := middleware.CurrentUserID(c)

	// Rate-limit: 3 suggestions per user per day
	rlKey := "rl:suggestion:" + userID.String()
	count, err := redisrepo.Increment(c.Request.Context(), h.rdb, rlKey, 24*time.Hour)
	if err == nil && count > 3 {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "лимит предложений на сегодня исчерпан (макс. 3 в день)"})
		return
	}

	var req struct {
		Body string `json:"body" binding:"required,min=1,max=3000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "текст предложения не может быть пустым"})
		return
	}

	s, err := h.repo.CreateSuggestion(c.Request.Context(), body)
	if err != nil {
		h.log.Error("create suggestion failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось сохранить предложение"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": s.ID, "message": "предложение принято, спасибо!"})
}
