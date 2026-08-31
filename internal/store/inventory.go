package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type InventoryStore struct {
	db *sqlx.DB
}

func NewInventoryStore(db *sqlx.DB) *InventoryStore {
	return &InventoryStore{db: db}
}

func (s *InventoryStore) GetStock(ctx context.Context, productID int64) (int64, error) {
	var quantity int64
	query := `SELECT quantity FROM inventory WHERE product_id = $1`
	err := s.db.QueryRowxContext(ctx, query, productID).Scan(&quantity)
	return quantity, err
}

// Reserve уменьшает остаток. Возвращает true, если списание успешно, и false, если остатка недостаточно.
func (s *InventoryStore) Reserve(ctx context.Context, productID int64, quantity int32) (bool, error) {
	query := `UPDATE inventory SET quantity = quantity - $1 WHERE product_id = $2 AND quantity >= $1`
	res, err := s.db.ExecContext(ctx, query, quantity, productID)
	if err != nil {
		return false, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}
