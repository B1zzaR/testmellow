package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vpnplatform/internal/domain"
)

// GET /api/admin/suggestions?status=&limit=&offset=
func (h *Handler) ListSuggestions(c *gin.Context) {
	status := c.Query("status")
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	items, err := h.repo.ListSuggestions(c.Request.Context(), status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load suggestions"})
		return
	}
	if items == nil {
		items = []*domain.Suggestion{}
	}
	c.JSON(http.StatusOK, gin.H{"suggestions": items})
}

// PATCH /api/admin/suggestions/:id
func (h *Handler) UpdateSuggestionStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	allowed := map[string]bool{"new": true, "read": true, "archived": true}
	if !allowed[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status value"})
		return
	}

	if err := h.repo.UpdateSuggestionStatus(
		c.Request.Context(), id, domain.SuggestionStatus(req.Status),
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update suggestion"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "status updated"})
}
