package main

import (
	"log"
	"log/slog"
	"net"
	"os"

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
	defer db.Close()

	slog.Info("Inventory Service connected to PostgreSQL")

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen on port 50051: %v", err)
	}

	grpcServer := grpc.NewServer()

	inventoryServer := inventory.NewServer(db)

	pb.RegisterInventoryServiceServer(grpcServer, inventoryServer)

	slog.Info("Inventory gRPC Server starting on :50051")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC: %v", err)
	}
}
