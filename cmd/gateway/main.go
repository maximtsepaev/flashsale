package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	invClient "github.com/maximtsepaev/flashsale/internal/client/inventory"
	"github.com/maximtsepaev/flashsale/internal/handler"
	"github.com/maximtsepaev/flashsale/internal/kafka"
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
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("Failed to close DB connection", "error", err)
		}
	}()

	grpcTarget := os.Getenv("INVENTORY_GRPC_URL")
	if grpcTarget == "" {
		grpcTarget = "localhost:50051"
	}

	client, conn, err := invClient.NewInventoryClient(grpcTarget)
	if err != nil {
		log.Fatalf("Failed to create inventory gRPC client: %v", err)
	}
	defer conn.Close()

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9092"
	}

	orderProducer := kafka.NewProducer(kafkaBroker, "orders")
	defer func() {
		if err := orderProducer.Close(); err != nil {
			slog.Error("Failed to close Kafka producer", "error", err)
		}
	}()
	slog.Info("Successfully initialized Kafka Producer", "broker", kafkaBroker)

	h := handler.NewHandler(db, client, orderProducer)
	router := h.InitRoutes()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		slog.Info("Gateway HTTP Server starting on :8080")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Gateway HTTP server failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down Gateway HTTP Server gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Gateway forced to shutdown", "error", err)
	}

	slog.Info("Gateway exited cleanly")
}
