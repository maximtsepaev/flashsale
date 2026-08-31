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
	"github.com/maximtsepaev/flashsale/internal/store"
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
	slog.Info("Gateway connected to PostgreSQL")

	grpcTarget := os.Getenv("INVENTORY_GRPC_URL")
	if grpcTarget == "" {
		grpcTarget = "localhost:50051"
	}

	client, conn, err := invClient.NewInventoryClient(grpcTarget)
	if err != nil {
		log.Fatalf("Failed to create inventory gRPC client: %v", err)
	}
	defer conn.Close()
	slog.Info("Connected to Inventory gRPC service", "target", grpcTarget)

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

	userStore := store.NewUserStore(db)
	h := handler.NewHandler(userStore, client, orderProducer)
	router := h.InitRoutes()

	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + httpPort,
		Handler: router,
	}

	go func() {
		slog.Info("Gateway HTTP Server starting", "port", httpPort)
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
