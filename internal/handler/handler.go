package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/maximtsepaev/flashsale/internal/middleware"
	pb "github.com/maximtsepaev/flashsale/internal/pb/inventory"
)

type Handler struct {
	db              *sqlx.DB
	inventoryClient pb.InventoryServiceClient
}

func NewHandler(db *sqlx.DB, inventoryClient pb.InventoryServiceClient) *Handler {
	return &Handler{
		db:              db,
		inventoryClient: inventoryClient,
	}
}

func (h *Handler) InitRoutes() *gin.Engine {
	r := gin.New()

	r.Use(middleware.LoggerMiddleware())

	r.POST("/register", h.handleRegister)
	r.POST("/login", h.handleLogin)

	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.POST("/orders", h.handleCreateOrder)
	}
	return r
}
