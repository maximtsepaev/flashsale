package inventory

import (
	"context"

	pb "github.com/maximtsepaev/flashsale/internal/pb/inventory"
	"github.com/maximtsepaev/flashsale/internal/store"
)

type Server struct {
	pb.UnimplementedInventoryServiceServer
	inventoryStore *store.InventoryStore
}

func NewServer(inventoryStore *store.InventoryStore) *Server {
	return &Server{inventoryStore: inventoryStore}
}

// GetStock возвращает текущее количество товара из базы данных
func (s *Server) GetStock(ctx context.Context, req *pb.GetStockRequest) (*pb.GetStockResponse, error) {
	quantity, err := s.inventoryStore.GetStock(ctx, req.ProductId)
	if err != nil {
		return nil, err
	}
	return &pb.GetStockResponse{Quantity: quantity}, nil
}

// ReserveProduct атомарно уменьшает остаток товара в базе данных
func (s *Server) ReserveProduct(ctx context.Context, req *pb.ReserveRequest) (*pb.ReserveResponse, error) {
	success, err := s.inventoryStore.Reserve(ctx, req.ProductId, req.Quantity)
	if err != nil {
		return nil, err
	}
	if !success {
		return &pb.ReserveResponse{Success: false, Message: "Not enough stock"}, nil
	}
	return &pb.ReserveResponse{Success: true, Message: "Success"}, nil
}
