package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scfabricio/redis-app/internal/domain"
)

var ErrNotFound = errors.New("brand not found")

type BrandRepository interface {
	FindByID(context.Context, int64) (domain.Brand, error)
	ListIDs(context.Context, int) ([]int64, error)
}

type PostgresBrandRepository struct{ pool *pgxpool.Pool }

func NewPostgresBrandRepository(pool *pgxpool.Pool) *PostgresBrandRepository {
	return &PostgresBrandRepository{pool: pool}
}

func (r *PostgresBrandRepository) FindByID(ctx context.Context, id int64) (domain.Brand, error) {
	var brand domain.Brand
	err := r.pool.QueryRow(ctx, `SELECT id, name, country, created_at FROM brands WHERE id = $1`, id).
		Scan(&brand.ID, &brand.Name, &brand.Country, &brand.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Brand{}, ErrNotFound
	}
	if err != nil {
		return domain.Brand{}, fmt.Errorf("query brand: %w", err)
	}
	return brand, nil
}

func (r *PostgresBrandRepository) ListIDs(ctx context.Context, limit int) ([]int64, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM brands ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list brand ids: %w", err)
	}
	defer rows.Close()
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
