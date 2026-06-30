package inventory

import (
	"context"

	"github.com/jmoiron/sqlx"
	pb "github.com/maximtsepaev/flashsale/internal/pb/inventory"
)

type Server struct {
	pb.UnimplementedInventoryServiceServer
	db *sqlx.DB
}

func NewServer(db *sqlx.DB) *Server {
	return &Server{db: db}
}

// GetStock возвращает текущее количество товара из базы данных
func (s *Server) GetStock(ctx context.Context, req *pb.GetStockRequest) (*pb.GetStockResponse, error) {
	productId := req.ProductId
	query := "SELECT quantity FROM inventory WHERE product_id = $1"
	var quantity int64

	err := s.db.QueryRowxContext(ctx, query, productId).Scan(&quantity)
	if err != nil {
		return nil, err
	}

	return &pb.GetStockResponse{Quantity: quantity}, nil
}

// ReserveProduct атомарно уменьшает остаток товара в базе данных
func (s *Server) ReserveProduct(ctx context.Context, req *pb.ReserveRequest) (*pb.ReserveResponse, error) {
	query := "UPDATE inventory SET quantity = quantity - $1 WHERE product_id = $2 AND quantity >= $1"

	res, err := s.db.ExecContext(ctx, query, req.Quantity, req.ProductId)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return &pb.ReserveResponse{Success: false, Message: "Not enough stock"}, nil
	} else {
		return &pb.ReserveResponse{Success: true, Message: "Success"}, nil
	}
}
