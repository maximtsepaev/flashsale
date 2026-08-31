package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type OrderStore struct {
	db *sqlx.DB
}

func NewOrderStore(db *sqlx.DB) *OrderStore {
	return &OrderStore{db: db}
}

// CreateOrder сохраняет заказ в базу данных. Возвращает ошибку, если что-то пошло не так.
func (s *OrderStore) CreateOrder(ctx context.Context, userID, productID int64, quantity int32, status string) error {
	query := `INSERT INTO orders (user_id, product_id, quantity, status) VALUES ($1, $2, $3, $4)`
	_, err := s.db.ExecContext(ctx, query, userID, productID, quantity, status)
	return err
}
