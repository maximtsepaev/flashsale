package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

	c.JSON(http.StatusCreated, gin.H{
		"message": "Order created successfully",
	})
}
