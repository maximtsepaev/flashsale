package inventory

import (
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

// func (s *Server) GetStock(productID int64) (int, error) {}

// func (s *Server) ReserveProduct(productID int64, quantity int) error {}
