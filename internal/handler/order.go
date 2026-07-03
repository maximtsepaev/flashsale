package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/maximtsepaev/flashsale/internal/kafka"
	pb "github.com/maximtsepaev/flashsale/internal/pb/inventory"
)

type OrderRequest struct {
	ProductID int64 `json:"product_id" binding:"required"`
	Quantity  int32 `json:"quantity" binding:"required,gt=0"`
}

func (h *Handler) handleCreateOrder(c *gin.Context) {
	var orderRequest OrderRequest
	if err := c.ShouldBindJSON(&orderRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID, ok := userIDVal.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	req := &pb.ReserveRequest{
		ProductId: orderRequest.ProductID,
		Quantity:  orderRequest.Quantity,
	}

	resp, err := h.inventoryClient.ReserveProduct(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal infrastructure error",
			"details": err.Error(),
		})
		return
	}

	if !resp.Success {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to reserve product",
			"message": resp.GetMessage(),
		})
		return
	}

	kafkaMsg := kafka.OrderMessage{
		UserID:    userID,
		ProductID: orderRequest.ProductID,
		Quantity:  orderRequest.Quantity,
	}

	err = h.kafkaProducer.PublishOrder(c.Request.Context(), kafkaMsg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to process order asynchronously",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Order created successfully",
	})
}
