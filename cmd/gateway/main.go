package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	invClient "github.com/maximtsepaev/flashsale/internal/client/inventory"
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
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer db.Close()

	grpcTarget := os.Getenv("INVENTORY_GRPC_URL")
	if grpcTarget == "" {
		grpcTarget = "localhost:50051"
	}

	client, conn, err := invClient.NewInventoryClient(grpcTarget)
	if err != nil {
		log.Fatalf("Failed to create inventory gRPC client: %v", err)
	}
	defer conn.Close()

	h := handler.NewHandler(db, client)
	router := h.InitRoutes()

	slog.Info("Gateway HTTP Server starting on :8080")
	if err := router.Run(":8080"); err != nil {
		slog.Error("Server failed to start", "error", err)
	}
}
