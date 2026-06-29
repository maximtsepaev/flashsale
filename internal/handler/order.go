package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maximtsepaev/flashsale/internal/middleware"
)

func (h *Handler) handleCreateOrder(c *gin.Context) {
	userID := c.MustGet(middleware.UserIDKey).(int64)

	c.JSON(http.StatusOK, gin.H{
		"message": "Order endpoint working",
		"user_id": userID,
	})
}
