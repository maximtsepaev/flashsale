package store

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

type User struct {
	ID           int64     `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	CreatedAt    time.Time `db:"created_at"`
}

type UserStore struct {
	db *sqlx.DB
}

func NewUserStore(db *sqlx.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) CreateUser(ctx context.Context, email, passwordHash string) (int64, error) {
	var id int64
	query := "INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id"
	err := s.db.QueryRowxContext(ctx, query, email, passwordHash).Scan(&id)
	return id, err
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	query := "SELECT id, email, password_hash, created_at FROM users WHERE email = $1"
	err := s.db.GetContext(ctx, &user, query, email)
	if err != nil {
		return nil, err
	}

	return &user, err
}
