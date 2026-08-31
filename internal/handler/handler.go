package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/maximtsepaev/flashsale/internal/kafka"
	"github.com/maximtsepaev/flashsale/internal/middleware"
	pb "github.com/maximtsepaev/flashsale/internal/pb/inventory"
	"github.com/maximtsepaev/flashsale/internal/store"
)

type Handler struct {
	userStore       *store.UserStore // <-- заменили db на userStore
	inventoryClient pb.InventoryServiceClient
	kafkaProducer   *kafka.Producer
}

func NewHandler(userStore *store.UserStore, inventoryClient pb.InventoryServiceClient, kafkaProducer *kafka.Producer) *Handler {
	return &Handler{
		userStore:       userStore,
		inventoryClient: inventoryClient,
		kafkaProducer:   kafkaProducer,
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
