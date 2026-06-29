package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/maximtsepaev/flashsale/internal/middleware"
)

type Handler struct {
	db *sqlx.DB
}

func NewHandler(db *sqlx.DB) *Handler {
	return &Handler{db: db}
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
