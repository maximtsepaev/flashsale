package main

import (
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"

	pb "github.com/maximtsepaev/flashsale/internal/pb/inventory"
	"github.com/maximtsepaev/flashsale/internal/services/inventory"
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
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("Failed to close DB connection", "error", err)
		}
	}()

	slog.Info("Inventory Service connected to PostgreSQL")

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen on port 50051: %v", err)
	}

	grpcServer := grpc.NewServer()
	inventoryServer := inventory.NewServer(db)
	pb.RegisterInventoryServiceServer(grpcServer, inventoryServer)

	go func() {
		slog.Info("Inventory gRPC Server starting on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("Inventory gRPC server stopped", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down Inventory gRPC Server gracefully...")
	grpcServer.GracefulStop()
	slog.Info("Inventory Service exited cleanly")
}
