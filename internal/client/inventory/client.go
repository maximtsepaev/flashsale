package inventory

import (
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/maximtsepaev/flashsale/internal/pb/inventory"
)

// NewInventoryClient открывает соединение с gRPC сервером и возвращает готовый клиент
func NewInventoryClient(target string) (pb.InventoryServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}

	slog.Info("Successfully initialized gRPC client for Inventory Service", "target", target)

	return pb.NewInventoryServiceClient(conn), conn, nil
}
