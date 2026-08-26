package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
)

type OrderMessage struct {
	UserID    int64 `json:"user_id"`
	ProductID int64 `json:"product_id"`
	Quantity  int32 `json:"quantity"`
}

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
	slog.Info("Order Service connected to PostgreSQL")

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9092"
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{kafkaBroker},
		Topic:    "orders",
		GroupID:  "order-processors",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})
	slog.Info("Order Service Kafka Reader started", "broker", kafkaBroker)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		slog.Info("Order Service is running and waiting for messages...")

		for {
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
					slog.Info("Kafka consumer loop stopped")
					return
				}
				slog.Error("Failed to read message from Kafka", "error", err)
				return
			}

			var orderMsg OrderMessage
			if err := json.Unmarshal(msg.Value, &orderMsg); err != nil {
				slog.Error("Failed to unmarshal order message", "error", err)
				continue
			}

			slog.Info("Received new order message from Kafka",
				"user_id", orderMsg.UserID,
				"product_id", orderMsg.ProductID,
				"quantity", orderMsg.Quantity,
			)

			query := `INSERT INTO orders (user_id, product_id, quantity, status) VALUES ($1, $2, $3, $4)`
			_, err = db.Exec(query, orderMsg.UserID, orderMsg.ProductID, orderMsg.Quantity, "created")
			if err != nil {
				slog.Error("Failed to insert order into database", "error", err)
				continue
			}

			slog.Info("Successfully saved order to database", "user_id", orderMsg.UserID)
		}
	}()

	<-ctx.Done()
	slog.Info("Order Service gracefully shutting down...")

	if err := reader.Close(); err != nil {
		slog.Error("Failed to close Kafka reader", "error", err)
	}

	wg.Wait()
	slog.Info("Order Service exited cleanly")
}
