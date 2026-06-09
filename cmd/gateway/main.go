package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	// Импортируем новые востребованные библиотеки
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Драйвер остается

	"github.com/maximtsepaev/flashsale/internal/auth"
)

// Изменяем тип глобальной переменной на sqlx.DB
var db *sqlx.DB

// Описываем структуру запроса. Gin умеет автоматически проверять поля с помощью тега binding.
type UserRequest struct {
	Email    string `json:"email" binding:"required,email"`    // Обязательное поле + валидация формата email
	Password string `json:"password" binding:"required,min=6"` // Пароль минимум 6 символов
}

func main() {
	// Инициализация логгера
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	var err error
	// МЕНТОРСКАЯ ПОДСКАЗКА: sqlx.Connect делает сразу две вещи под капотом:
	// 1. sql.Open (инициализирует пул)
	// 2. db.Ping (проверяет реальное соединение). Ручной Ping больше не нужен!
	db, err = sqlx.Connect("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to DB via sqlx: %v", err)
	}
	defer db.Close()

	slog.Info("Successfully connected to PostgreSQL via sqlx")

	// Инициализируем роутер Gin вместо http.NewServeMux()
	// gin.Default() автоматически включает в себя Middleware для логирования и Recovery (чтобы сервер не падал при панике)
	r := gin.Default()

	// Публичные маршруты
	r.POST("/register", handleRegister)
	r.POST("/login", handleLogin)

	// Защищенные маршруты. На следующем этапе мы перепишем твой Middleware под Gin,
	// а пока регистрируем эндпоинт так.
	protected := r.Group("/")
	// TODO: тут будет наш новый AuthMiddleware для Gin
	protected.POST("/orders", handleCreateOrder)

	slog.Info("Server starting on :8080")
	// Запуск сервера встроенным методом Gin
	if err := r.Run(":8080"); err != nil {
		slog.Error("Server failed to start", "error", err)
	}
}

func handleRegister(c *gin.Context) {
	var req UserRequest

	// МЕНТОРСКАЯ ПОДСКАЗКА: Метод ShouldBindJSON делает всю грязную работу за тебя.
	// Он читает c.Request.Body, парсит JSON в структуру и запускает валидацию (binding).
	if err := c.ShouldBindJSON(&req); err != nil {
		// Если email невалидный или пароль короткий — Gin сразу возвращает красивый JSON с ошибкой
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	var id int64
	query := "INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id"

	// Используем db.QueryRowx() от sqlx — это расширенная версия стандартного QueryRow
	err = db.QueryRowx(query, req.Email, hash).Scan(&id)
	if err != nil {
		// В проде лучше проверять конкретный код ошибки Postgres (Unique Violation),
		// но для базового уровня пока вернем общую ошибку конфликта
		c.JSON(http.StatusConflict, gin.H{"error": "user already exists or DB error"})
		return
	}

	// Отправляем структурированный JSON ответ вместо простой строки
	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user_id": id,
	})
}

func handleLogin(c *gin.Context) {
	var req UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var id int64
	var hash string
	query := "SELECT id, password_hash FROM users WHERE email = $1"

	err := db.QueryRowx(query, req.Email).Scan(&id, &hash)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	if !auth.CheckPasswordHash(req.Password, hash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	token, err := auth.GenerateToken(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func handleCreateOrder(c *gin.Context) {
	// Пока заглушка, к ней вернемся, когда перепишем Middleware
	c.JSON(http.StatusOK, gin.H{"message": "Order endpoint"})
}
