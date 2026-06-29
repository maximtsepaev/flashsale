package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/maximtsepaev/flashsale/internal/handler"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to DB via sqlx: %v", err)
	}
	defer db.Close()

	slog.Info("Successfully connected to PostgreSQL via sqlx")

	h := handler.NewHandler(db)

	router := h.InitRoutes()

	slog.Info("Server starting on :8080")
	if err := router.Run(":8080"); err != nil {
		slog.Error("Server failed to start", "error", err)
	}
}
